package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyBearerRejectsForgedIdentity(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer valid" {
			w.WriteHeader(401)
			return
		}
		w.Write([]byte(`{"authenticated":true,"userId":7,"role":"user"}`))
	}))
	defer auth.Close()
	handler := VerifyBearer(auth.URL)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-ID") != "7" || r.Header.Get("X-User-Role") != "user" {
			t.Fatal("forged identity survived")
		}
		w.WriteHeader(204)
	}))
	for _, token := range []string{"", "Bearer bad", "Bearer valid"} {
		req := httptest.NewRequest("GET", "/v1/study-sets/1/play", nil)
		req.Header.Set("X-User-ID", "99")
		req.Header.Set("X-User-Role", "admin")
		req.Header.Set("Authorization", token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		expected := 401
		if token == "Bearer valid" {
			expected = 204
		}
		if w.Code != expected {
			t.Fatalf("token %q status=%d", token, w.Code)
		}
	}
	auth.Close()
	req := httptest.NewRequest("GET", "/v1/study-sets/1/play", nil)
	req.Header.Set("Authorization", "Bearer valid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 503 {
		t.Fatal("auth outage opened access")
	}
}
