package http

import (
	"strconv"
	"testing"
	"time"
)

func newTestLoginGuard() *loginGuard {
	guard := newLoginGuard()
	guard.now = func() time.Time { return time.Unix(1700000000, 0) }
	return guard
}

func advance(guard *loginGuard, d time.Duration) {
	base := guard.now()
	guard.now = func() time.Time { return base.Add(d) }
}

func TestLoginGuardBlocksAfterFiveFailuresForIP(t *testing.T) {
	guard := newTestLoginGuard()
	for i := 0; i < loginAccountIPLimit-1; i++ {
		guard.RecordFailure("User@Example.com", "198.51.100.1")
		if blocked, _ := guard.Blocked("user@example.com", "198.51.100.1"); blocked {
			t.Fatalf("blocked after %d failures, want %d", i+1, loginAccountIPLimit)
		}
	}
	guard.RecordFailure("User@Example.com", "198.51.100.1")

	blocked, retry := guard.Blocked("user@example.com", "198.51.100.1")
	if !blocked {
		t.Fatal("must block once the failure budget is spent")
	}
	if retry <= 0 || retry > loginLockMax {
		t.Fatalf("unexpected retry-after %s", retry)
	}
	// The same account from another address is not blocked by this rule alone.
	if blocked, _ := guard.Blocked("user@example.com", "198.51.100.2"); blocked {
		t.Fatal("another IP must not share the per-IP lock")
	}
}

func TestLoginGuardBlocksAccountAcrossManyIPs(t *testing.T) {
	guard := newTestLoginGuard()
	for i := 0; i < loginAccountLimit; i++ {
		guard.RecordFailure("victim@example.com", "198.51.100."+strconv.Itoa(i))
	}
	blocked, _ := guard.Blocked("victim@example.com", "203.0.113.77")
	if !blocked {
		t.Fatal("rotating IPs must still trip the account-wide ceiling")
	}
}

func TestLoginGuardLockGrowsProgressivelyAndExpires(t *testing.T) {
	guard := newTestLoginGuard()
	for i := 0; i < loginAccountIPLimit; i++ {
		guard.RecordFailure("a@example.com", "198.51.100.1")
	}
	_, firstWait := guard.Blocked("a@example.com", "198.51.100.1")
	if firstWait != loginLockBase {
		t.Fatalf("first lock = %s, want %s", firstWait, loginLockBase)
	}

	guard.RecordFailure("a@example.com", "198.51.100.1")
	_, secondWait := guard.Blocked("a@example.com", "198.51.100.1")
	if secondWait <= firstWait {
		t.Fatalf("second lock (%s) must be longer than the first (%s)", secondWait, firstWait)
	}

	advance(guard, loginLockMax)
	if blocked, _ := guard.Blocked("a@example.com", "198.51.100.1"); blocked {
		t.Fatal("lock must expire")
	}
}

func TestLoginGuardSuccessClearsFailures(t *testing.T) {
	guard := newTestLoginGuard()
	for i := 0; i < loginAccountIPLimit; i++ {
		guard.RecordFailure("b@example.com", "198.51.100.1")
	}
	guard.RecordSuccess("b@example.com", "198.51.100.1")
	if blocked, _ := guard.Blocked("b@example.com", "198.51.100.1"); blocked {
		t.Fatal("successful login must clear the failure budget")
	}
}

func TestLoginGuardWindowResetsFailures(t *testing.T) {
	guard := newTestLoginGuard()
	for i := 0; i < loginAccountIPLimit-1; i++ {
		guard.RecordFailure("c@example.com", "198.51.100.1")
	}
	advance(guard, loginAccountIPWindow+time.Minute)
	guard.RecordFailure("c@example.com", "198.51.100.1")
	if blocked, _ := guard.Blocked("c@example.com", "198.51.100.1"); blocked {
		t.Fatal("failures outside the window must not count")
	}
}

func TestLoginGuardIgnoresEmptyEmail(t *testing.T) {
	guard := newTestLoginGuard()
	guard.RecordFailure("   ", "198.51.100.1")
	if blocked, _ := guard.Blocked("", "198.51.100.1"); blocked {
		t.Fatal("empty email must not be tracked")
	}
}
