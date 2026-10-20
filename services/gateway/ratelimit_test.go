package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// freshRateLimits swaps in an isolated configuration for the duration of a test.
func freshRateLimits(t *testing.T) *rateLimitConfig {
	t.Helper()
	original := rateLimits
	cfg := &rateLimitConfig{
		enabled:  true,
		limiter:  newMemoryLimiter(100),
		failures: newFailureTracker(100),
	}
	rateLimits = cfg
	t.Cleanup(func() { rateLimits = original })
	return cfg
}

// withRules temporarily narrows the package rule table so tests stay fast.
func withRules(t *testing.T, applied func()) {
	t.Helper()
	snapshot := struct {
		login, reauth, joinIP, joinCode, apiUser Rule
	}{ruleAuthLogin, ruleAuthReauth, ruleLiveJoinIP, ruleLiveJoinCode, ruleAPIUser}
	t.Cleanup(func() {
		ruleAuthLogin, ruleAuthReauth = snapshot.login, snapshot.reauth
		ruleLiveJoinIP, ruleLiveJoinCode = snapshot.joinIP, snapshot.joinCode
		ruleAPIUser = snapshot.apiUser
	})
	applied()
}

func TestMemoryLimiterRefillsOverWindow(t *testing.T) {
	now := time.Unix(1700000000, 0)
	limiter := newMemoryLimiter(10)
	limiter.now = func() time.Time { return now }
	rule := Rule{Name: "test", Limit: 2, Window: time.Minute}

	if ok, _ := limiter.Allow("ip:1", rule); !ok {
		t.Fatal("first request must pass")
	}
	if ok, _ := limiter.Allow("ip:1", rule); !ok {
		t.Fatal("second request must pass")
	}
	ok, retry := limiter.Allow("ip:1", rule)
	if ok {
		t.Fatal("third request must be limited")
	}
	if retry <= 0 || retry > time.Minute {
		t.Fatalf("unexpected retry-after %s", retry)
	}

	// Half the window refills one token at 2/minute.
	now = now.Add(30 * time.Second)
	if ok, _ := limiter.Allow("ip:1", rule); !ok {
		t.Fatal("token should have refilled after half a window")
	}
	// A full window later the bucket is full again.
	now = now.Add(time.Minute)
	for i := 0; i < 2; i++ {
		if ok, _ := limiter.Allow("ip:1", rule); !ok {
			t.Fatalf("request %d after refill must pass", i+1)
		}
	}
}

func TestMemoryLimiterKeysAreIndependent(t *testing.T) {
	limiter := newMemoryLimiter(10)
	rule := Rule{Limit: 1, Window: time.Minute}
	if ok, _ := limiter.Allow("ip:1", rule); !ok {
		t.Fatal("first key must pass")
	}
	if ok, _ := limiter.Allow("ip:1", rule); ok {
		t.Fatal("same key must be limited")
	}
	if ok, _ := limiter.Allow("ip:2", rule); !ok {
		t.Fatal("different key must not be affected")
	}
}

func TestMemoryLimiterEvictsIdleAndCapsKeys(t *testing.T) {
	now := time.Unix(1700000000, 0)
	limiter := newMemoryLimiter(5)
	limiter.now = func() time.Time { return now }
	rule := Rule{Limit: 100, Window: time.Minute}

	for i := 0; i < 5; i++ {
		limiter.Allow("k"+strconv.Itoa(i), rule)
	}
	// Move past the idle TTL so the next insert can reclaim space.
	now = now.Add(11 * time.Minute)
	if ok, _ := limiter.Allow("fresh", rule); !ok {
		t.Fatal("request must still pass during eviction")
	}
	if got := limiter.size(); got > 5 {
		t.Fatalf("limiter grew past its cap: %d keys", got)
	}

	// Flooding unique keys must never grow the map without bound.
	for i := 0; i < 200; i++ {
		limiter.Allow("flood-"+strconv.Itoa(i), rule)
	}
	if got := limiter.size(); got > 5 {
		t.Fatalf("limiter grew past its cap under flood: %d keys", got)
	}
}

func TestFailureTrackerBlocksUntilWindowEnds(t *testing.T) {
	now := time.Unix(1700000000, 0)
	tracker := newFailureTracker(10)
	tracker.now = func() time.Time { return now }
	window := time.Hour

	for i := 0; i < 3; i++ {
		tracker.RecordFailure("badcode:1.2.3.4", window)
	}
	if blocked, _ := tracker.Blocked("badcode:1.2.3.4", 4, window); blocked {
		t.Fatal("must not block below the limit")
	}
	tracker.RecordFailure("badcode:1.2.3.4", window)
	blocked, retry := tracker.Blocked("badcode:1.2.3.4", 4, window)
	if !blocked {
		t.Fatal("must block once the limit is reached")
	}
	if retry <= 0 || retry > window {
		t.Fatalf("unexpected retry-after %s", retry)
	}
	if blocked, _ := tracker.Blocked("badcode:5.6.7.8", 4, window); blocked {
		t.Fatal("other keys must not be blocked")
	}

	now = now.Add(window)
	if blocked, _ := tracker.Blocked("badcode:1.2.3.4", 4, window); blocked {
		t.Fatal("block must expire with the window")
	}
}

func TestFailureTrackerStopsTrackingBeyondCap(t *testing.T) {
	tracker := newFailureTracker(2)
	for i := 0; i < 50; i++ {
		tracker.RecordFailure("k"+strconv.Itoa(i), time.Hour)
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if len(tracker.entries) > 2 {
		t.Fatalf("failure tracker grew past its cap: %d entries", len(tracker.entries))
	}
}

func TestFailureTrackerResetClearsKey(t *testing.T) {
	tracker := newFailureTracker(10)
	tracker.RecordFailure("k", time.Hour)
	tracker.reset("k")
	if blocked, _ := tracker.Blocked("k", 1, time.Hour); blocked {
		t.Fatal("reset must clear failures")
	}
}

func TestMatchPublicRuleSelectsSpecificRoutes(t *testing.T) {
	for _, tt := range []struct {
		method, path, want string
	}{
		{"POST", "/v1/auth/login", ruleAuthLogin.Name},
		{"POST", "/v1/auth/register", ruleAuthRegister.Name},
		// Live join is enforced inside its proxy because it needs the room code.
		{"POST", "/v1/live-sessions/ABC123/join", ""},
		{"GET", "/v1/auth/login", ""},
		{"POST", "/v1/auth/logout", ""},
		{"POST", "/v1/live-sessions/1/answers", ""},
	} {
		rule, ok := matchPublicRule(tt.method, tt.path)
		if tt.want == "" {
			if ok {
				t.Fatalf("%s %s unexpectedly matched %s", tt.method, tt.path, rule.Name)
			}
			continue
		}
		if !ok || rule.Name != tt.want {
			t.Fatalf("%s %s matched %q, want %q", tt.method, tt.path, rule.Name, tt.want)
		}
	}
}

func TestLiveSessionID(t *testing.T) {
	for path, want := range map[string]string{
		"/v1/live-sessions/ABC123/join": "ABC123",
		"/v1/live-sessions/42/answers":  "42",
		"/v1/live-sessions/42":          "42",
		"/v1/study-sets/1":              "",
		"/v1/live-sessions/":            "",
	} {
		if got := liveSessionID(path); got != want {
			t.Fatalf("liveSessionID(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestIPRateLimitBlocksLoginAfterThreshold(t *testing.T) {
	freshRateLimits(t)
	withRules(t, func() {
		ruleAuthLogin = Rule{Name: "auth.login.ip", Limit: 2, Window: time.Minute}
	})

	upstreamCalled := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalled++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := requestID(rateLimits.ipRateLimit(reverseProxy(upstream.URL)))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		req.RemoteAddr = "198.51.100.1:1111"
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, resp.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	req.RemoteAddr = "198.51.100.1:1111"
	req.Header.Set("X-Request-ID", "req-rl")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.Code)
	}
	if retry := resp.Header().Get("Retry-After"); retry == "" || retry == "0" {
		t.Fatalf("expected Retry-After header, got %q", retry)
	}
	var body errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "RATE_LIMITED" || body.RequestID != "req-rl" {
		t.Fatalf("unexpected error envelope: %+v", body)
	}
	if upstreamCalled != 2 {
		t.Fatalf("upstream must not be called once limited, got %d calls", upstreamCalled)
	}

	// A different IP keeps its own budget.
	req = httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	req.RemoteAddr = "198.51.100.2:1111"
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("independent IP must still pass, got %d", resp.Code)
	}
}

func TestIPRateLimitIgnoresSpoofedForwardedForFromDirectClient(t *testing.T) {
	freshRateLimits(t)
	withRules(t, func() {
		ruleAuthLogin = Rule{Name: "auth.login.ip", Limit: 1, Window: time.Minute}
	})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := rateLimits.ipRateLimit(reverseProxy(upstream.URL))
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	req.RemoteAddr = "198.51.100.5:1111"
	req.Header.Set("X-Forwarded-For", "spoof-"+strconv.Itoa(len(t.Name())))
	handler.ServeHTTP(httptest.NewRecorder(), req)

	req = httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	req.RemoteAddr = "198.51.100.5:2222"
	req.Header.Set("X-Forwarded-For", "another")
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed header must not reset the budget, got %d", resp.Code)
	}
}

func TestBearerRequestsUseTokenBudgetNotSharedIP(t *testing.T) {
	freshRateLimits(t)
	withRules(t, func() {
		ruleAPIUser = Rule{Name: "api.user", Limit: 1, Window: time.Minute}
	})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := rateLimits.ipRateLimit(reverseProxy(upstream.URL))
	call := func(token string) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		req.RemoteAddr = "198.51.100.44:1111"
		req.Header.Set("Authorization", "Bearer "+token)
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		return resp.Code
	}
	if code := call("token-a"); code != http.StatusOK {
		t.Fatalf("first request got %d", code)
	}
	if code := call("token-a"); code != http.StatusTooManyRequests {
		t.Fatalf("second request on the same token got %d, want 429", code)
	}
	// A second user behind the same IP keeps a separate budget.
	if code := call("token-b"); code != http.StatusOK {
		t.Fatalf("second token got %d, want 200", code)
	}
}

func TestShadowModeLogsButAllows(t *testing.T) {
	cfg := freshRateLimits(t)
	cfg.shadow = true
	withRules(t, func() {
		ruleAuthLogin = Rule{Name: "auth.login.ip", Limit: 1, Window: time.Minute}
	})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := rateLimits.ipRateLimit(reverseProxy(upstream.URL))
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		req.RemoteAddr = "198.51.100.9:1111"
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("shadow mode must not block, request %d got %d", i+1, resp.Code)
		}
	}
	cfg.shadow = false
}

func TestDisabledRateLimitingAllowsEverything(t *testing.T) {
	cfg := freshRateLimits(t)
	cfg.enabled = false
	withRules(t, func() {
		ruleAuthLogin = Rule{Name: "auth.login.ip", Limit: 1, Window: time.Minute}
	})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := rateLimits.ipRateLimit(reverseProxy(upstream.URL))
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
		req.RemoteAddr = "198.51.100.10:1111"
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("disabled limiter blocked request %d with %d", i+1, resp.Code)
		}
	}
}

func TestAuthenticatedProxyAppliesPerUserLimit(t *testing.T) {
	freshRateLimits(t)
	withRules(t, func() {
		ruleAPIUser = Rule{Name: "api.user", Limit: 2, Window: time.Minute}
	})
	authCache = newTokenCache(60 * time.Second)
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(verifiedIdentity{Authenticated: true, UserID: 42})
	}))
	defer auth.Close()

	upstreamCalled := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalled++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	handler := requestID(authenticatedProxy(auth.URL, upstream.URL))
	call := func() int {
		req := httptest.NewRequest(http.MethodGet, "/v1/study-sets", nil)
		req.Header.Set("Authorization", "Bearer user-limit-token")
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		return resp.Code
	}
	if code := call(); code != http.StatusOK {
		t.Fatalf("first call got %d", code)
	}
	if code := call(); code != http.StatusOK {
		t.Fatalf("second call got %d", code)
	}
	if code := call(); code != http.StatusTooManyRequests {
		t.Fatalf("third call must be rate limited, got %d", code)
	}
	if upstreamCalled != 2 {
		t.Fatalf("upstream should see only the allowed calls, got %d", upstreamCalled)
	}
}

func TestJoinProxyLocksIPAfterTooManyWrongCodes(t *testing.T) {
	freshRateLimits(t)
	withRules(t, func() {
		ruleLiveJoinIP = Rule{Name: "live.join.ip", Limit: 1000, Window: time.Minute, Burst: 1000}
		ruleLiveJoinCode = Rule{Name: "live.join.code", Limit: 100, Window: time.Minute, Burst: 100}
	})
	upstreamCalled := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		upstreamCalled++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":"LIVE_SESSION_NOT_FOUND"}`))
	}))
	defer upstream.Close()

	handler := rateLimits.ipRateLimit(rateLimitProxy(upstream.URL))
	call := func(code, ip string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/live-sessions/"+code+"/join", nil)
		req.RemoteAddr = ip
		resp := httptest.NewRecorder()
		handler.ServeHTTP(resp, req)
		return resp
	}

	for i := 0; i < badCodeLimit; i++ {
		if resp := call("AAAAA"+strconv.Itoa(i%10), "198.51.100.30:1111"); resp.Code != http.StatusNotFound {
			t.Fatalf("guess %d: expected upstream 404, got %d", i+1, resp.Code)
		}
	}
	before := upstreamCalled
	resp := call("ZZZZZZ", "198.51.100.30:1111")
	if resp.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burning the wrong-code budget, got %d", resp.Code)
	}
	if upstreamCalled != before {
		t.Fatal("locked-out IP must not reach the quiz service")
	}
	var body errorEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "RATE_LIMITED" {
		t.Fatalf("unexpected envelope: %+v", body)
	}

	// A distinct IP is unaffected.
	if resp := call("ZZZZZZ", "198.51.100.31:1111"); resp.Code != http.StatusNotFound {
		t.Fatalf("independent IP got %d, want upstream 404", resp.Code)
	}
}

func TestRateLimitedMetricsExposed(t *testing.T) {
	freshRateLimits(t)
	rateLimitStats = &rateLimitMetric{counts: map[string]uint64{}}
	rule := Rule{Name: "auth.login.ip", Limit: 1, Window: time.Minute}
	if allowed, _ := rateLimits.check("POST /v1/auth/login", "ip:6.6.6.6", rule); !allowed {
		t.Fatal("first request must pass")
	}
	rateLimits.check("POST /v1/auth/login", "ip:6.6.6.6", rule)

	resp := httptest.NewRecorder()
	rateLimitStats.writePrometheus(resp, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if got := resp.Header().Get("Content-Type"); got != "text/plain; version=0.0.4" {
		t.Fatalf("unexpected content type %q", got)
	}
	if !containsMetric(resp.Body.String(), "rate_limited_total", "rule=\"auth.login.ip\"", "mode=\"blocked\"") {
		t.Fatalf("missing rate_limited_total series:\n%s", resp.Body.String())
	}
}

func containsMetric(body string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(body, part) {
			return false
		}
	}
	return true
}
