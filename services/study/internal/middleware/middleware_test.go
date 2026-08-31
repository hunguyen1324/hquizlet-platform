package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireUserID(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	h := RequireUserID(next)

	tests := []struct {
		name       string
		path       string
		userID     string
		wantStatus int
	}{
		{name: "health is public", path: "/healthz", wantStatus: http.StatusNoContent},
		{name: "missing user", path: "/v1/study-sets", wantStatus: http.StatusUnauthorized},
		{name: "invalid user", path: "/v1/study-sets", userID: "abc", wantStatus: http.StatusUnauthorized},
		{name: "zero user", path: "/v1/study-sets", userID: "0", wantStatus: http.StatusUnauthorized},
		{name: "negative user", path: "/v1/study-sets", userID: "-1", wantStatus: http.StatusUnauthorized},
		{name: "valid user", path: "/v1/study-sets", userID: "42", wantStatus: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.userID != "" {
				r.Header.Set("X-User-ID", tt.userID)
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, r)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}
