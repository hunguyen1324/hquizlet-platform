package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthenticatedProxyUsesVerifiedIdentity(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/auth/verify" { t.Fatalf("unexpected auth path: %s", r.URL.Path) }
		if r.Header.Get("Authorization") != "Bearer valid" { t.Fatalf("authorization was not forwarded") }
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(verifiedIdentity{Authenticated: true, UserID: 42, Email: "a@example.com", Name: "A", Role: "user"})
	}))
	defer auth.Close()

	study := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-User-ID"); got != "42" { t.Fatalf("expected verified user id 42, got %q", got) }
		if got := r.Header.Get("X-Request-ID"); got != "req-42" { t.Fatalf("expected request id propagation, got %q", got) }
		w.WriteHeader(http.StatusOK)
	}))
	defer study.Close()

	h := requestID(authenticatedProxy(auth.URL, study.URL))
	req := httptest.NewRequest(http.MethodGet, "/v1/study-sets/7/progress", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-User-ID", "999")
	req.Header.Set("X-Request-ID", "req-42")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK { t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String()) }
}

func TestAuthenticatedProxyRejectsMissingBearer(t *testing.T) {
	h := authenticatedProxy("http://127.0.0.1:1", "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodGet, "/v1/study-sets/7/progress", nil)
	req.Header.Set("X-Request-ID", "req-missing")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized { t.Fatalf("expected 401, got %d", resp.Code) }
	var body errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil { t.Fatal(err) }
	if body.Code != "UNAUTHORIZED" || body.Message == "" || body.RequestID != "req-missing" { t.Fatalf("unexpected error envelope: %+v", body) }
}
