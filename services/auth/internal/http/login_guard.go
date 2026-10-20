package http

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Failed-login guard.
//
// The gateway can only rate limit logins per IP, so a single account can still
// be brute-forced from many addresses. This guard counts failures per
// (account, IP) and per account, then waits progressively longer. It never
// reports whether the account exists: blocked and rejected logins look the same.
const (
	loginAccountIPLimit  = 5
	loginAccountIPWindow = 15 * time.Minute
	// Higher, IP-independent ceiling so rotating addresses does not give an
	// attacker unlimited guesses. Kept high to avoid trivially locking a user out.
	loginAccountLimit  = 20
	loginAccountWindow = 15 * time.Minute
	loginLockBase      = 30 * time.Second
	loginLockMax       = 15 * time.Minute
	loginGuardMaxKeys  = 10_000
)

type loginEntry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

type loginGuard struct {
	mu      sync.Mutex
	entries map[string]loginEntry
	now     func() time.Time
}

var failedLogins = newLoginGuard()

func newLoginGuard() *loginGuard {
	return &loginGuard{entries: map[string]loginEntry{}, now: time.Now}
}

func accountIPKey(email, ip string) string { return "acct-ip|" + email + "|" + ip }
func accountKey(email string) string       { return "acct|" + email }

func normalizeLoginEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Blocked reports how long the caller must wait, if the account (or the
// account from this IP) is currently locked.
func (g *loginGuard) Blocked(email, ip string) (bool, time.Duration) {
	email = normalizeLoginEmail(email)
	if email == "" {
		return false, 0
	}
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, key := range []string{accountIPKey(email, ip), accountKey(email)} {
		entry, ok := g.entries[key]
		if !ok || entry.lockedUntil.IsZero() || !now.Before(entry.lockedUntil) {
			continue
		}
		wait := entry.lockedUntil.Sub(now)
		if wait < time.Second {
			wait = time.Second
		}
		return true, wait
	}
	return false, 0
}

// RecordFailure adds one failed attempt and locks the key when its budget is spent.
func (g *loginGuard) RecordFailure(email, ip string) {
	email = normalizeLoginEmail(email)
	if email == "" {
		return
	}
	now := g.now()
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pruneLocked(now)
	g.recordLocked(accountIPKey(email, ip), now, loginAccountIPLimit, loginAccountIPWindow, loginLockBase)
	g.recordLocked(accountKey(email), now, loginAccountLimit, loginAccountWindow, 5*time.Minute)
}

// RecordSuccess clears the failure budget for this account.
func (g *loginGuard) RecordSuccess(email, ip string) {
	email = normalizeLoginEmail(email)
	if email == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.entries, accountIPKey(email, ip))
	delete(g.entries, accountKey(email))
}

func (g *loginGuard) recordLocked(key string, now time.Time, limit int, window, lockBase time.Duration) {
	entry, ok := g.entries[key]
	if !ok || now.Sub(entry.windowStart) >= window {
		entry = loginEntry{windowStart: now}
	}
	entry.failures++
	if entry.failures >= limit {
		over := entry.failures - limit
		lock := lockBase * time.Duration(math.Pow(2, float64(over)))
		if lock > loginLockMax {
			lock = loginLockMax
		}
		until := now.Add(lock)
		if until.After(entry.lockedUntil) {
			entry.lockedUntil = until
		}
	}
	g.entries[key] = entry
}

// pruneLocked keeps the map bounded; entries older than their windows carry no
// information, so they can be dropped at any time.
func (g *loginGuard) pruneLocked(now time.Time) {
	if len(g.entries) < loginGuardMaxKeys {
		return
	}
	for key, entry := range g.entries {
		if now.Sub(entry.windowStart) >= loginAccountWindow && !now.Before(entry.lockedUntil) {
			delete(g.entries, key)
		}
	}
}

// writeLoginRateLimited answers 429 without revealing whether the account exists.
func writeLoginRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, apiError{
		Code:      "RATE_LIMITED",
		Message:   "too many failed login attempts, try again later",
		RequestID: requestID(w),
		Details:   map[string]any{"retryAfter": seconds},
	})
}
