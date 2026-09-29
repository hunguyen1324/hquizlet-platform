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
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/storage"
	_ "github.com/jackc/pgx/v5/stdlib"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

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
	audio := storage.NewQuizAudio(db, "")
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
	New(service.NewStudySetService(sets, cards), nil, nil, nil, service.NewQuizQuestionService(questions, sets), nil, nil, db).WithProtectedQuiz(service.NewProtectedQuizService(sets, questions, sessions, payment.URL), audio).Register(mux)
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
	session := base + "/play/" + view.ID
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
		{"bytes=0-3", 206, audioData[:4]}, {"bytes=4-7", 206, audioData[4:8]}, {"bytes=99999-", 416, nil},
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
