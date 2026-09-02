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

// ─── Phase 4: routeStudySets ─────────────────────────────────────────────────

func TestRouteStudySetsSendsQuizToQuizService(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(verifiedIdentity{Authenticated: true, UserID: 42})
	}))
	defer auth.Close()

	studyCalled := false
	study := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		studyCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer study.Close()

	quizCalled := false
	quiz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quizCalled = true
		if got := r.Header.Get("X-User-ID"); got != "42" {
			t.Fatalf("expected verified user id 42 in quiz service, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer quiz.Close()

	h := requestID(routeStudySets(auth.URL, study.URL, quiz.URL))

	// /v1/study-sets/101/quiz/generate should go to quiz service
	req := httptest.NewRequest(http.MethodPost, "/v1/study-sets/101/quiz/generate", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-Request-ID", "req-quiz")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !quizCalled {
		t.Fatal("quiz service should have been called")
	}
	if studyCalled {
		t.Fatal("study service should NOT have been called")
	}
}

func TestRouteStudySetsSendsProgressToStudyService(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(verifiedIdentity{Authenticated: true, UserID: 42})
	}))
	defer auth.Close()

	studyCalled := false
	study := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		studyCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer study.Close()

	quizCalled := false
	quiz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quizCalled = true
	}))
	defer quiz.Close()

	h := requestID(routeStudySets(auth.URL, study.URL, quiz.URL))

	// /v1/study-sets/101/progress should go to study service
	req := httptest.NewRequest(http.MethodGet, "/v1/study-sets/101/progress", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-Request-ID", "req-progress")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !studyCalled {
		t.Fatal("study service should have been called")
	}
	if quizCalled {
		t.Fatal("quiz service should NOT have been called")
	}
}

func TestRouteStudySetsRejectsMissingBearer(t *testing.T) {
	h := routeStudySets("http://127.0.0.1:1", "http://127.0.0.1:1", "http://127.0.0.1:1")
	req := httptest.NewRequest(http.MethodPost, "/v1/study-sets/101/quiz/generate", nil)
	req.Header.Set("X-Request-ID", "req-noauth")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

// ─── Phase 7: Class Routes ──────────────────────────────────────────────────

func TestClassRoutesRequireAuth(t *testing.T) {
	class := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer class.Close()

	h := authenticatedProxy("http://127.0.0.1:1", class.URL)

	// GET /v1/classes without auth should return 401
	req := httptest.NewRequest(http.MethodGet, "/v1/classes", nil)
	req.Header.Set("X-Request-ID", "req-noauth")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestClassRoutesStripsSpoofedHeaders(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(verifiedIdentity{Authenticated: true, UserID: 42, Email: "a@example.com", Name: "A", Role: "user"})
	}))
	defer auth.Close()

	class := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-User-ID"); got != "42" {
			t.Fatalf("expected verified user id 42, got %q", got)
		}
		// Verify spoofed headers are stripped
		if got := r.Header.Get("X-Class-Role"); got != "" {
			t.Fatalf("expected X-Class-Role to be stripped, got %q", got)
		}
		if got := r.Header.Get("X-Member-ID"); got != "" {
			t.Fatalf("expected X-Member-ID to be stripped, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer class.Close()

	h := requestID(authenticatedProxy(auth.URL, class.URL))
	req := httptest.NewRequest(http.MethodGet, "/v1/classes", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-User-ID", "999") // spoofed
	req.Header.Set("X-Class-Role", "admin") // spoofed
	req.Header.Set("X-Member-ID", "123") // spoofed
	req.Header.Set("X-Request-ID", "req-class")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestActivityRouteRequiresAuth(t *testing.T) {
	class := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer class.Close()

	h := authenticatedProxy("http://127.0.0.1:1", class.URL)

	req := httptest.NewRequest(http.MethodGet, "/v1/activity", nil)
	req.Header.Set("X-Request-ID", "req-activity-noauth")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
}

func TestClassRoutesForwardToClassService(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(verifiedIdentity{Authenticated: true, UserID: 42})
	}))
	defer auth.Close()

	classCalled := false
	class := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		classCalled = true
		if got := r.Header.Get("X-User-ID"); got != "42" {
			t.Fatalf("expected verified user id 42 in class service, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer class.Close()

	h := requestID(authenticatedProxy(auth.URL, class.URL))

	req := httptest.NewRequest(http.MethodGet, "/v1/classes", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-Request-ID", "req-class-list")
	resp := httptest.NewRecorder()
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if !classCalled {
		t.Fatal("class service should have been called")
	}
}

func TestRouteStudySetsStripsSpoofedUserID(t *testing.T) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(verifiedIdentity{Authenticated: true, UserID: 99})
	}))
	defer auth.Close()

	quiz := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-User-ID"); got != "99" {
			t.Fatalf("expected spoofed X-User-ID to be replaced with verified 99, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer quiz.Close()

	h := requestID(routeStudySets(auth.URL, "http://127.0.0.1:1", quiz.URL))
	req := httptest.NewRequest(http.MethodPost, "/v1/study-sets/101/quiz/generate", nil)
	req.Header.Set("Authorization", "Bearer valid")
	req.Header.Set("X-User-ID", "12345") // spoofed
	req.Header.Set("X-Request-ID", "req-spoof")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
}
