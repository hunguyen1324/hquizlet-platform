package http

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/migration"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/snapshotcache"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/storage"
)

func TestListeningPostgresHTTP(t *testing.T) {
	dsn := os.Getenv("QUIZ_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("QUIZ_TEST_DATABASE_URL not configured")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i := 0; i < 2; i++ {
		if err = migration.Run(db); err != nil {
			t.Fatal(err)
		}
	}
	uid := time.Now().UnixNano() / 1000
	var setID, otherSet int64
	for _, target := range []*int64{&setID, &otherSet} {
		if err = db.QueryRow(`INSERT INTO study_sets(user_id,title,content_type,visibility) VALUES($1,'Listening QA','quiz','private') RETURNING id`, uid).Scan(target); err != nil {
			t.Fatal(err)
		}
	}
	defer db.Exec(`DELETE FROM study_sets WHERE id IN($1,$2)`, setID, otherSet)
	defer db.Exec(`DELETE FROM protected_quiz_limits WHERE user_id=$1`, uid)
	sets := repository.NewStudySetRepository(db)
	cards := repository.NewFlashcardRepository(db)
	questions := repository.NewQuizQuestionRepository(db)
	audio := storage.NewQuizAudio(db, "").WithDiskCache(t.TempDir(), 1<<20)
	raw := []byte("ID3\x04\x00\x00\x00\x00\x00\x00TEST_AUDIO")
	ref, err := audio.Put(context.Background(), setID, bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("d", 32)
	srt := "1\n00:00:00,000 --> 00:00:01,000\nXin chào\n"
	stt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer private-stt-token" {
			t.Error("STT authorization missing")
		}
		if r.Method == "POST" {
			body, _ := io.ReadAll(r.Body)
			if !bytes.Equal(body, raw) || r.ContentLength != int64(len(raw)) {
				t.Error("audio stream changed")
			}
			WriteJSON(w, 202, map[string]string{"id": id, "status": "queued"})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/srt") {
			w.Write([]byte(srt))
			return
		}
		WriteJSON(w, 200, map[string]string{"id": id, "status": "completed"})
	}))
	defer stt.Close()
	protected := service.NewProtectedQuizService(sets, questions, repository.NewQuizSessionRepository(db), snapshotcache.New(repository.NewQuizSnapshotRepository(db), snapshotcache.DefaultBytes), "")
	mux := http.NewServeMux()
	New(service.NewStudySetService(sets, cards), nil, nil, nil, service.NewQuizQuestionService(questions, sets), nil, nil, db).WithProtectedQuiz(protected, audio).WithSTT(stt.URL, "private-stt-token").Register(mux)
	call := func(method, path string, user int64, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-User-ID", fmt.Sprint(user))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	base := fmt.Sprintf("/v1/study-sets/%d", setID)
	payload, _ := json.Marshal(map[string]string{"audioUrl": ref, "language": "vi"})
	if w := call("POST", base+"/listening/transcriptions", uid+1, string(payload)); w.Code != 403 {
		t.Fatalf("foreign owner accepted: %d", w.Code)
	}
	if w := call("GET", base+"/listening/audio?source="+ref, uid+1, ""); w.Code != 403 {
		t.Fatal("preview leaked")
	}
	if w := call("GET", base+"/listening/audio?source="+ref, uid, ""); w.Code != 200 || !bytes.Equal(w.Body.Bytes(), raw) {
		t.Fatalf("preview %d", w.Code)
	}
	if w := call("POST", base+"/listening/transcriptions", uid, string(payload)); w.Code != 202 {
		t.Fatalf("submit %d %s", w.Code, w.Body.String())
	}
	if w := call("GET", fmt.Sprintf("/v1/study-sets/%d/listening/transcriptions/%s", otherSet, id), uid, ""); w.Code != 404 {
		t.Fatalf("cross-set job exposed: %d", w.Code)
	}
	w := call("GET", base+"/listening/transcriptions/"+id, uid, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Xin chào") {
		t.Fatalf("result %d %s", w.Code, w.Body.String())
	}
	w = call("GET", base+"/listening/transcriptions?audioUrl="+ref, uid, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), id) {
		t.Fatal("cannot resume latest job")
	}
	input := model.BulkSaveQuizQuestionsInput{Questions: []model.CreateQuizQuestionInput{{QuestionText: "Nghe và chọn đáp án", QuestionType: "written", AudioURL: &ref, SRTText: srt}}}
	data, _ := json.Marshal(input)
	w = call("PUT", base+"/quiz-questions", uid, string(data))
	if w.Code != 200 {
		t.Fatalf("save %d %s", w.Code, w.Body.String())
	}
	saved, err := questions.ListByStudySet(context.Background(), setID)
	if err != nil || len(saved) != 1 || saved[0].SRTText != srt {
		t.Fatalf("subtitle persistence failed: %+v %v", saved, err)
	}
}
