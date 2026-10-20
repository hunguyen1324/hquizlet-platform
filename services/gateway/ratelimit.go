package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Limiter ────────────────────────────────────────────────────────────────
//
// Rate limiting is a token bucket: a bucket of `Burst` tokens refilled at
// `Limit` per `Window`. One token is spent per request. The in-process
// implementation is enough for a single gateway; the Limiter interface is the
// seam for a Redis-backed implementation when more replicas are added.

// Rule is one rate-limit ceiling.
type Rule struct {
	// Name identifies the rule in logs and metrics, e.g. "auth.login.ip".
	Name string
	// Limit is the number of requests allowed per Window.
	Limit int
	// Window is the refill period.
	Window time.Duration
	// Burst is the bucket capacity; defaults to Limit.
	Burst int
}

func (r Rule) burst() int {
	if r.Burst > 0 {
		return r.Burst
	}
	if r.Limit > 0 {
		return r.Limit
	}
	return 1
}

func (r Rule) ratePerSecond() float64 {
	if r.Window <= 0 {
		return 1
	}
	return float64(r.Limit) / r.Window.Seconds()
}

// Limiter decides whether a key may make one more request.
type Limiter interface {
	Allow(key string, rule Rule) (allowed bool, retryAfter time.Duration)
}

type bucket struct {
	tokens float64
	last   time.Time
	seen   time.Time
}

type memoryLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	maxKeys int
	now     func() time.Time
}

func newMemoryLimiter(maxKeys int) *memoryLimiter {
	if maxKeys <= 0 {
		maxKeys = 100_000
	}
	return &memoryLimiter{buckets: map[string]*bucket{}, maxKeys: maxKeys, now: time.Now}
}

func (m *memoryLimiter) Allow(key string, rule Rule) (bool, time.Duration) {
	if rule.Limit <= 0 || rule.Window <= 0 {
		return true, 0
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()

	b, ok := m.buckets[key]
	if !ok {
		m.evictLocked(now)
		b = &bucket{tokens: float64(rule.burst()), last: now}
		m.buckets[key] = b
	} else if elapsed := now.Sub(b.last).Seconds(); elapsed > 0 {
		b.tokens = math.Min(float64(rule.burst()), b.tokens+elapsed*rule.ratePerSecond())
		b.last = now
	}
	b.seen = now

	if b.tokens >= 1 {
		b.tokens--
		return true, 0
	}
	wait := time.Duration((1 - b.tokens) / rule.ratePerSecond() * float64(time.Second))
	if wait < time.Second {
		wait = time.Second
	}
	return false, wait
}

// evictLocked keeps memory bounded: drop idle buckets first, then the least
// recently seen ones when an attacker floods unique keys.
func (m *memoryLimiter) evictLocked(now time.Time) {
	if len(m.buckets) < m.maxKeys {
		return
	}
	const idleTTL = 10 * time.Minute
	for key, b := range m.buckets {
		if now.Sub(b.seen) > idleTTL {
			delete(m.buckets, key)
		}
	}
	if len(m.buckets) < m.maxKeys {
		return
	}
	target := m.maxKeys / 10
	removed := 0
	for key := range m.buckets {
		delete(m.buckets, key)
		removed++
		if removed >= target {
			return
		}
	}
}

func (m *memoryLimiter) size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.buckets)
}

// failureTracker counts failures inside a fixed window and blocks the key for
// the rest of that window once the limit is reached. Used for "wrong join code"
// attempts, where what matters is how many guesses happened, not their rate.
type failureTracker struct {
	mu      sync.Mutex
	entries map[string]failureEntry
	maxKeys int
	now     func() time.Time
}

type failureEntry struct {
	windowStart time.Time
	count       int
}

func newFailureTracker(maxKeys int) *failureTracker {
	if maxKeys <= 0 {
		maxKeys = 100_000
	}
	return &failureTracker{entries: map[string]failureEntry{}, maxKeys: maxKeys, now: time.Now}
}

// Blocked reports whether the key already used up its failure budget.
func (f *failureTracker) Blocked(key string, limit int, window time.Duration) (bool, time.Duration) {
	if limit <= 0 || window <= 0 {
		return false, 0
	}
	now := f.now()
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, ok := f.entries[key]
	if !ok || now.Sub(entry.windowStart) >= window {
		return false, 0
	}
	if entry.count < limit {
		return false, 0
	}
	wait := window - now.Sub(entry.windowStart)
	if wait < time.Second {
		wait = time.Second
	}
	return true, wait
}

// RecordFailure adds one failure for the key.
func (f *failureTracker) RecordFailure(key string, window time.Duration) {
	now := f.now()
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, ok := f.entries[key]
	if !ok {
		if len(f.entries) >= f.maxKeys {
			for k, existing := range f.entries {
				if now.Sub(existing.windowStart) >= window {
					delete(f.entries, k)
				}
			}
		}
		// Still full: stop tracking new keys instead of growing without bound.
		if len(f.entries) >= f.maxKeys {
			return
		}
	}
	if !ok || now.Sub(entry.windowStart) >= window {
		entry = failureEntry{windowStart: now}
	}
	entry.count++
	f.entries[key] = entry
}

func (f *failureTracker) reset(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.entries, key)
}

// ─── Rules ──────────────────────────────────────────────────────────────────

var (
	// Public (unauthenticated) rules, keyed by client IP.
	ruleAuthLogin    = Rule{Name: "auth.login.ip", Limit: 10, Window: time.Minute}
	ruleAuthRegister = Rule{Name: "auth.register.ip", Limit: 5, Window: time.Hour}
	ruleAuthReauth   = Rule{Name: "auth.reauth.ip", Limit: 10, Window: time.Minute}
	ruleLiveJoinIP   = Rule{Name: "live.join.ip", Limit: 20, Window: time.Minute}
	// One join request may use several room codes; this stops code enumeration.
	ruleLiveJoinCode = Rule{Name: "live.join.code", Limit: 10, Window: time.Minute}
	// Broad ceiling for everything else from a single IP (NAT/classroom friendly).
	ruleDefaultIP = Rule{Name: "api.ip", Limit: 900, Window: time.Minute, Burst: 120}

	// Authenticated rules, keyed by user id.
	ruleAPIUser = Rule{Name: "api.user", Limit: 300, Window: time.Minute, Burst: 60}
	// Admin console keeps its own tighter budget (reads/writes split).
	ruleAdminRead  = Rule{Name: "admin.read.user", Limit: 120, Window: time.Minute}
	ruleAdminWrite = Rule{Name: "admin.write.user", Limit: 20, Window: time.Minute}
	// Live player actions (answers, player-state), keyed by IP + session.
	ruleLivePlayer = Rule{Name: "live.player.ip", Limit: 60, Window: time.Minute}

	// Wrong room code budget: 60 bad guesses per IP per hour.
	badCodeLimit  = 60
	badCodeWindow = time.Hour
)

type publicRule struct {
	rule  *Rule
	match func(method, path string) bool
}

func exactPath(method, path string) func(string, string) bool {
	return func(m, p string) bool { return m == method && p == path }
}

// publicRules is ordered: the first match wins, so specific routes come first.
// Live join is enforced inside rateLimitProxy instead, because it also needs the
// room code from the path and must count rejected codes.
var publicRules = []publicRule{
	{&ruleAuthLogin, exactPath(http.MethodPost, "/v1/auth/login")},
	{&ruleAuthRegister, exactPath(http.MethodPost, "/v1/auth/register")},
	{&ruleAuthReauth, exactPath(http.MethodPost, "/v1/auth/reauth")},
}

func matchPublicRule(method, path string) (Rule, bool) {
	for _, entry := range publicRules {
		if entry.match(method, path) {
			return *entry.rule, true
		}
	}
	return Rule{}, false
}

// exemptFromDefaultIP reports paths where the broad per-IP ceiling does not
// apply: health probes, metrics, and long-lived SSE connections (nginx limits
// those by concurrent connection instead of request rate).
func exemptFromDefaultIP(r *http.Request) bool {
	path := r.URL.Path
	if path == "/healthz" || path == "/healthz/services" || path == "/metrics" {
		return true
	}
	return r.Method == http.MethodGet && strings.HasSuffix(path, "/events")
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	switch strings.ToLower(raw) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}

// ─── Configuration and enforcement ──────────────────────────────────────────

type rateLimitConfig struct {
	enabled  bool
	shadow   bool
	limiter  Limiter
	failures *failureTracker
}

func loadRateLimitConfig() *rateLimitConfig {
	maxKeys := envInt("RATE_LIMIT_MAX_KEYS", 100_000)
	cfg := &rateLimitConfig{
		enabled:  envBool("RATE_LIMIT_ENABLED", true),
		shadow:   envBool("RATE_LIMIT_SHADOW", false),
		limiter:  newMemoryLimiter(maxKeys),
		failures: newFailureTracker(maxKeys),
	}
	switch {
	case !cfg.enabled:
		log.Printf("[gateway] rate limiting disabled (RATE_LIMIT_ENABLED=false)")
	case cfg.shadow:
		log.Printf("[gateway] rate limiting in shadow mode: violations are logged, never blocked")
	default:
		log.Printf("[gateway] rate limiting enabled")
	}
	return cfg
}

// rateLimits is the process-wide configuration, initialised from the env once.
var rateLimits = loadRateLimitConfig()

// check applies a bucket rule, honouring the global off switch and shadow mode.
func (c *rateLimitConfig) check(route, key string, rule Rule) (bool, time.Duration) {
	if c == nil || !c.enabled {
		return true, 0
	}
	// Namespace the bucket by rule so the same IP or user can hold an
	// independent budget per route group (login vs. generic API vs. join).
	allowed, retry := c.limiter.Allow(rule.Name+"|"+key, rule)
	if allowed {
		return true, 0
	}
	if c.shadow {
		rateLimitStats.record(route, rule.Name, "shadow")
		log.Printf("[gateway] rate_limit_shadow method_route=%s rule=%s key=%s retry_after=%s", route, rule.Name, key, retry.Round(time.Second))
		return true, 0
	}
	rateLimitStats.record(route, rule.Name, "blocked")
	log.Printf("[gateway] rate_limited route=%s rule=%s key=%s retry_after=%s", route, rule.Name, key, retry.Round(time.Second))
	return false, retry
}

// badCodeBlocked reports whether the IP exhausted its wrong-room-code budget.
func (c *rateLimitConfig) badCodeBlocked(route, ip string) (bool, time.Duration) {
	if c == nil || !c.enabled {
		return false, 0
	}
	blocked, retry := c.failures.Blocked("badcode:"+ip, badCodeLimit, badCodeWindow)
	if !blocked {
		return false, 0
	}
	if c.shadow {
		rateLimitStats.record(route, "live.join.badcode", "shadow")
		log.Printf("[gateway] rate_limit_shadow route=%s rule=live.join.badcode key=badcode:%s retry_after=%s", route, ip, retry.Round(time.Second))
		return false, 0
	}
	rateLimitStats.record(route, "live.join.badcode", "blocked")
	log.Printf("[gateway] rate_limited route=%s rule=live.join.badcode key=badcode:%s retry_after=%s", route, ip, retry.Round(time.Second))
	return true, retry
}

func (c *rateLimitConfig) recordBadCode(ip string) {
	if c == nil || !c.enabled || ip == "" {
		return
	}
	c.failures.RecordFailure("badcode:"+ip, badCodeWindow)
}

// writeRateLimited answers 429 with Retry-After and the gateway error envelope.
func writeRateLimited(w http.ResponseWriter, r *http.Request, retry time.Duration) {
	seconds := int(math.Ceil(retry.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, errorEnvelope{
		Code:      "RATE_LIMITED",
		Message:   fmt.Sprintf("too many requests, retry in %ds", seconds),
		RequestID: r.Header.Get("X-Request-ID"),
		Details:   map[string]any{"retryAfter": seconds},
	})
}

// ─── Middleware ─────────────────────────────────────────────────────────────

// ipRateLimit applies the public per-IP rules before a request reaches any
// upstream service. Authenticated user budgets are enforced by the proxy
// helpers once the identity is known.
func (c *rateLimitConfig) ipRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIPFromRequest(r)
		if ip != "" {
			if !c.ipAllowed(w, r, ip) {
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (c *rateLimitConfig) ipAllowed(w http.ResponseWriter, r *http.Request, ip string) bool {
	if rule, ok := matchPublicRule(r.Method, r.URL.Path); ok {
		route := r.Method + " " + r.URL.Path
		allowed, retry := c.check(route, "ip:"+ip, rule)
		if !allowed {
			writeRateLimited(w, r, retry)
			return false
		}
		return true
	}
	if exemptFromDefaultIP(r) {
		return true
	}
	if token := bearerTokenValue(r); token != "" {
		// Credential-bound budget instead of the IP ceiling, so a shared
		// classroom/NAT address is not throttled as a whole. Routes that verify
		// the token apply the same ceiling again per user id.
		allowed, retry := c.check(r.Method+" default", "token:"+tokenFingerprint(token), ruleAPIUser)
		if !allowed {
			writeRateLimited(w, r, retry)
			return false
		}
		return true
	}
	allowed, retry := c.check(r.Method+" default", "ip:"+ip, ruleDefaultIP)
	if !allowed {
		writeRateLimited(w, r, retry)
		return false
	}
	return true
}

// allowUser enforces an authenticated (per-user) rule.
func (c *rateLimitConfig) allowUser(w http.ResponseWriter, r *http.Request, userID int64, rule Rule) bool {
	key := "user:" + strconv.FormatInt(userID, 10)
	allowed, retry := c.check(r.Method+" "+r.URL.Path, key, rule)
	if !allowed {
		writeRateLimited(w, r, retry)
		return false
	}
	return true
}

// allowParticipant enforces a live-player rule keyed by IP + session so one
// classroom IP cannot be starved by a single player.
func (c *rateLimitConfig) allowParticipant(w http.ResponseWriter, r *http.Request, rule Rule) bool {
	key := "session:" + liveSessionID(r.URL.Path) + ":ip:" + clientIPFromRequest(r)
	allowed, retry := c.check(r.Method+" "+r.URL.Path, key, rule)
	if !allowed {
		writeRateLimited(w, r, retry)
		return false
	}
	return true
}

// bearerTokenValue returns the raw bearer token, or "" when absent.
func bearerTokenValue(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}

// tokenFingerprint keeps raw credentials out of limiter keys, logs and metrics.
func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}

// liveSessionID extracts the {code} or {sessionId} segment of a live-session path.
func liveSessionID(path string) string {
	const prefix = "/v1/live-sessions/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := path[len(prefix):]
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}

// ─── Metrics ────────────────────────────────────────────────────────────────

type rateLimitMetric struct {
	mu     sync.Mutex
	counts map[string]uint64
}

var rateLimitStats = &rateLimitMetric{counts: map[string]uint64{}}

func (m *rateLimitMetric) record(route, rule, mode string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.counts) > 10_000 {
		m.counts = map[string]uint64{}
	}
	m.counts[route+"\x00"+rule+"\x00"+mode]++
}

// writePrometheus renders rate_limited_total{route,rule,mode} for scraping.
func (m *rateLimitMetric) writePrometheus(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for key, value := range m.counts {
		parts := strings.SplitN(key, "\x00", 3)
		if len(parts) != 3 {
			continue
		}
		_, _ = fmt.Fprintf(w, "rate_limited_total{route=%q,rule=%q,mode=%q} %d\n", parts[0], parts[1], parts[2], value)
	}
}
