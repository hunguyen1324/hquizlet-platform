package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Learners can update their own reviews without needing content-editor permission.
// Access to the set and the review's user identity are checked by the handler.
func TestSpacedRepetitionUsesLearnerPolicy(t *testing.T) {
	for _, method := range []string{"POST", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			called := false
			h := StudyWritePolicy(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) }))
			r := httptest.NewRequest(method, "/v1/study-sets/12/spaced-repetition", nil)
			r.Header.Set("X-User-ID", "7")
			r.Header.Set("X-User-Role", "student")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if !called || w.Code != 204 {
				t.Fatalf("learner review blocked: %d", w.Code)
			}
		})
	}
}
