package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestManagedRoleControlsStudyCreation(t *testing.T) {
	for _, tt := range []struct {
		role string
		want int
	}{{"Student", 204}, {"Teacher", 204}, {"PlatformSupport", 403}, {"", 403}} {
		h := StudyWritePolicy(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
		r := httptest.NewRequest("POST", "/v1/study-sets", nil)
		r.Header.Set("X-User-ID", "9")
		r.Header.Set("X-User-Roles", tt.role)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tt.want {
			t.Fatalf("role %q status=%d want=%d", tt.role, w.Code, tt.want)
		}
	}
}
