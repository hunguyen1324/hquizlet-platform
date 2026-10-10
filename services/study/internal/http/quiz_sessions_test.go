package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

func TestPlayErrorSessionQuota(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/v1/study-sets/42/play", nil)
	(&Handler{}).playError(w, r, repository.ErrQuizSessionLimit)
	if w.Code != 409 {
		t.Fatalf("status=%d", w.Code)
	}
	var body struct {
		Code        string
		StudySetID  int64 `json:"studySetId"`
		ActiveCount int   `json:"activeCount"`
		Limit       int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "quiz_session_limit" || body.StudySetID != 42 || body.ActiveCount != 5 || body.Limit != 5 {
		t.Fatalf("unexpected quota: %+v", body)
	}
}
