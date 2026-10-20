package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/migration"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/snapshotcache"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/storage"
	_ "github.com/jackc/pgx/v5/stdlib"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeObjectStore answers HEAD/GET with Range support, standing in for MinIO.
type fakeObjectStore struct {
	data   []byte
	mu     sync.Mutex
	ranges []string
}

func (f *fakeObjectStore) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start, end := int64(0), int64(len(f.data))-1
	ranged := false
	if spec := r.Header.Get("Range"); spec != "" {
		f.mu.Lock()
		f.ranges = append(f.ranges, spec)
		f.mu.Unlock()
		ranged = true
		parts := strings.Split(strings.TrimPrefix(spec, "bytes="), "-")
		if n, err := strconv.ParseInt(parts[0], 10, 64); err == nil && n <= end {
			start = n
		}
		if len(parts) == 2 && parts[1] != "" {
			if n, err := strconv.ParseInt(parts[1], 10, 64); err == nil && n < end {
				end = n
			}
		}
	}
	body := f.data[start : end+1]
	w.Header().Set("Content-Length", fmt.Sprint(len(body)))
	if ranged {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(f.data)))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	w.Write(body)
}

// Run only against a disposable database explicitly provided by the caller.
func TestProtectedQuizPostgresHTTP(t *testing.T) {
	dsn := os.Getenv("QUIZ_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("QUIZ_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migration.Run(db); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	uid := time.Now().UnixNano() / 1000
	var setID int64
	err = db.QueryRow(`INSERT INTO study_sets(user_id,title,description,content_type,visibility) VALUES($1,'Security integration test','','quiz','public') RETURNING id`, uid).Scan(&setID)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Exec(`DELETE FROM study_sets WHERE id=$1`, setID)
	defer db.Exec(`DELETE FROM protected_quiz_limits WHERE user_id=$1`, uid)
	sets := repository.NewStudySetRepository(db)
	cards := repository.NewFlashcardRepository(db)
	questions := repository.NewQuizQuestionRepository(db)
	sessions := repository.NewQuizSessionRepository(db)
	cacheDir := t.TempDir()
	audio := storage.NewQuizAudio(db, "").WithDiskCache(cacheDir, 0)
	audioData := []byte("ID3\x04\x00\x00\x00\x00\x00\x00TEST_AUDIO")
	ref, err := audio.Put(ctx, setID, bytes.NewReader(audioData))
	if err != nil {
		t.Fatal(err)
	}
	answer := "B"
	explanation := "private explanation"
	err = questions.BulkSave(ctx, setID, []model.CreateQuizQuestionInput{
		{QuestionText: "First", QuestionType: "multiple_choice", CorrectAnswer: &answer, AnswerExplanation: &explanation, AudioURL: &ref, Options: []model.CreateOptionInput{{Text: "Wrong", Position: 0}, {Text: "Right", Position: 1, IsCorrect: true}}},
		{QuestionText: "Second", QuestionType: "written", CorrectAnswer: &answer, Position: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uid
		switch r.Header.Get("Authorization") {
		case "Bearer owner":
		case "Bearer other":
			id++
		default:
			w.WriteHeader(401)
			return
		}
		fmt.Fprintf(w, `{"authenticated":true,"userId":%d}`, id)
	}))
	defer auth.Close()
	payment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"hasAccess":true}`)) }))
	defer payment.Close()
	mux := http.NewServeMux()
	snapshots := snapshotcache.New(repository.NewQuizSnapshotRepository(db), snapshotcache.DefaultBytes)
	New(service.NewStudySetService(sets, cards), nil, nil, nil, service.NewQuizQuestionService(questions, sets), nil, nil, db).WithProtectedQuiz(service.NewProtectedQuizService(sets, questions, sessions, snapshots, payment.URL), audio).Register(mux)
	server := httptest.NewServer(middleware.VerifyBearer(auth.URL)(mux))
	defer server.Close()
	request := func(method, path, token, body string) (int, []byte, http.Header) {
		t.Helper()
		req, _ := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		req.Header.Set("Authorization", token)
		req.Header.Set("X-User-ID", fmt.Sprint(uid))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, data, resp.Header
	}
	base := fmt.Sprintf("/v1/study-sets/%d", setID)
	if status, _, _ := request("GET", base+"/play", "", ""); status != 401 {
		t.Fatalf("forged identity status %d", status)
	}
	if status, _, _ := request("GET", base+"/quiz-questions", "Bearer other", ""); status != 403 {
		t.Fatalf("raw export accessible: %d", status)
	}
	if status, _, _ := request("GET", base+"/flashcards", "Bearer other", ""); status != 403 {
		t.Fatalf("flashcard bypass accessible: %d", status)
	}
	status, data, headers := request("POST", base+"/play", "Bearer owner", `{"mode":"exam","layout":"default","durationSeconds":60}`)
	if status != 201 {
		t.Fatalf("start status %d %s", status, data)
	}
	if headers.Get("Cache-Control") != "private, no-store" {
		t.Fatal("missing no-store")
	}
	for _, secret := range []string{"private-audio:", "correctAnswer", "private explanation", "Second"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("leak %s", data)
		}
	}
	var view model.QuizSessionView
	if err = json.Unmarshal(data, &view); err != nil {
		t.Fatal(err)
	}
	if view.Total != 2 || len(view.Manifest) != 2 {
		t.Fatalf("start payload lacks totals/manifest: %s", data)
	}
	// Questions live in one immutable snapshot; the mutable state stays small.
	var snapshotID string
	var hasItems bool
	if err = db.QueryRow(`SELECT snapshot_id, state ? 'items' FROM protected_quiz_sessions WHERE id=$1`, view.ID).Scan(&snapshotID, &hasItems); err != nil {
		t.Fatal(err)
	}
	if snapshotID == "" || hasItems {
		t.Fatalf("session was not split from its snapshot: snapshot=%q items=%v", snapshotID, hasItems)
	}
	session := base + "/play/" + view.ID
	// A page response no longer repeats the manifest; resuming asks for it once.
	status, pageData, _ := request("GET", session+"?index=0", "Bearer owner", "")
	if status != 200 || strings.Contains(string(pageData), `"manifest"`) || !strings.Contains(string(pageData), `"total":2`) {
		t.Fatalf("page response is not compact: %d %s", status, pageData)
	}
	status, resumeData, _ := request("GET", session+"?index=0&manifest=1", "Bearer owner", "")
	if status != 200 || !strings.Contains(string(resumeData), `"manifest"`) {
		t.Fatalf("resume did not include the manifest: %d %s", status, resumeData)
	}
	var resumed model.QuizSessionView
	if err = json.Unmarshal(resumeData, &resumed); err != nil || len(resumed.Manifest) != 2 {
		t.Fatalf("resume manifest: %v %s", err, resumeData)
	}
	if status, _, _ := request("GET", session, "Bearer other", ""); status != 404 {
		t.Fatalf("cross-user session access %d", status)
	}
	if status, _, _ := request("GET", session+"/audio?index=0", "Bearer other", ""); status != 404 {
		t.Fatalf("cross-user audio access %d", status)
	}
	status, data, headers = request("GET", session+"/audio?index=0", "Bearer owner", "")
	if status != 200 || !bytes.Equal(data, audioData) || headers.Get("Content-Type") != "audio/mpeg" {
		t.Fatalf("audio failed %d %s", status, data)
	}
	if headers.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("audio is not seekable: %q", headers.Get("Accept-Ranges"))
	}
	status, data, _ = request("GET", session+"/audio-ticket?index=0", "Bearer owner", "")
	var media struct {
		Path string `json:"path"`
	}
	if status != 200 || json.Unmarshal(data, &media) != nil || media.Path == "" {
		t.Fatalf("ticket failed: %d %s", status, data)
	}
	for _, span := range []struct {
		value  string
		status int
		body   []byte
	}{
		{"bytes=0-0", 206, audioData[:1]}, {"bytes=0-3", 206, audioData[:4]}, {"bytes=4-7", 206, audioData[4:8]}, {"bytes=99999-", 416, nil},
	} {
		req, _ := http.NewRequest("GET", server.URL+media.Path, nil)
		req.Header.Set("Range", span.value)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != span.status || (span.body != nil && !bytes.Equal(body, span.body)) {
			t.Fatalf("range %s: %d %q", span.value, resp.StatusCode, body)
		}
		if span.status == 206 && resp.Header.Get("Content-Range") == "" {
			t.Fatal("missing Content-Range")
		}
	}
	// The stream path no longer selects `body`: blank the column and play the
	// same audio again. It must come from the local copy made on the first read.
	var audioID string
	if err = db.QueryRow(`SELECT id FROM protected_quiz_audio WHERE study_set_id=$1`, setID).Scan(&audioID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE protected_quiz_audio SET body='' WHERE id=$1`, audioID); err != nil {
		t.Fatal(err)
	}
	status, data, _ = request("GET", session+"/audio?index=0", "Bearer owner", "")
	if status != 200 || !bytes.Equal(data, audioData) {
		t.Fatalf("audio needed the BYTEA column: %d %d bytes", status, len(data))
	}
	// With the local copy evicted, the same audio is streamed by byte range
	// from the object store while Postgres still holds only metadata.
	object := &fakeObjectStore{data: audioData}
	objectServer := httptest.NewServer(object)
	defer objectServer.Close()
	objects, err := storage.NewMinIOImportStorage(storage.MinIOConfig{Endpoint: objectServer.URL, AccessKeyID: "minioadmin", SecretAccessKey: "minioadmin", Bucket: "hquizlet"})
	if err != nil {
		t.Fatal(err)
	}
	audio.WithObjectStore(objects.(storage.ObjectRangeStore))
	if err = os.Remove(filepath.Join(cacheDir, audioID)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE protected_quiz_audio SET object_key=$2 WHERE id=$1`, audioID, "quiz-audio/"+audioID); err != nil {
		t.Fatal(err)
	}
	status, data, _ = request("GET", session+"/audio?index=0", "Bearer owner", "")
	if status != 200 || !bytes.Equal(data, audioData) {
		t.Fatalf("object store audio failed %d %d bytes", status, len(data))
	}
	object.mu.Lock()
	ranges := append([]string(nil), object.ranges...)
	object.mu.Unlock()
	if len(ranges) == 0 || !strings.HasPrefix(ranges[0], "bytes=0-") {
		t.Fatalf("audio was not fetched by range: %v", ranges)
	}
	// Prefetch a page that was never opened: the content arrives, the rights do not.
	status, data, headers = request("GET", session+"/peek?index=1", "Bearer owner", "")
	if status != 200 || !strings.Contains(string(data), "Second") {
		t.Fatalf("peek failed %d %s", status, data)
	}
	for _, secret := range []string{"correctAnswer", "answerExplanation", "private explanation", "audioUrl"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("peek leaked %s: %s", secret, data)
		}
	}
	if headers.Get("Server-Timing") == "" {
		t.Fatal("peek is missing Server-Timing")
	}
	if status, data, _ = request("POST", session+"/answer", "Bearer owner", `{"index":1,"answer":"B"}`); status != 422 {
		t.Fatalf("peek granted answer rights: %d %s", status, data)
	}
	status, data, _ = request("POST", session+"/answer", "Bearer owner", `{"index":0,"answer":"Right"}`)
	if status != 200 || strings.Contains(string(data), "correctAnswer") {
		t.Fatalf("answer failed/leaked %d %s", status, data)
	}
	// Simulate a late answer by moving the persisted deadline; browser time is irrelevant.
	_, err = db.Exec(`UPDATE protected_quiz_sessions SET state=jsonb_set(state,'{deadline}',to_jsonb(to_char(now()-interval '1 second','YYYY-MM-DD"T"HH24:MI:SS"Z"'))) WHERE id=$1`, view.ID)
	if err != nil {
		t.Fatal(err)
	}
	status, data, _ = request("POST", session+"/answer", "Bearer owner", `{"index":0,"answer":"Wrong"}`)
	if status != 200 {
		t.Fatalf("deadline status %d %s", status, data)
	}
	json.Unmarshal(data, &view)
	if !view.Submitted || view.Score == nil || *view.Score != 1 || view.Answers[0] != "Right" {
		t.Fatalf("late answer changed grade: %s", data)
	}
	// Shared DB quota holds even when requests use separate repository instances.
	for i := 0; i < 3; i++ {
		err = repository.NewQuizSessionRepository(db).Allow(ctx, uid, "integration", 2, time.Hour)
		if (i < 2 && err != nil) || (i == 2 && err != repository.ErrQuizRateLimit) {
			t.Fatalf("quota request %d: %v", i, err)
		}
	}
	// A fresh session survives construction of another service/repository instance.
	_, err = sessions.Recent(ctx, uid, setID)
	if err != nil {
		t.Fatal(err)
	}
}
