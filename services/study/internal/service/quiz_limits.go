package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// QuizLimiter bounds how often a learner may hit a hot quiz endpoint.
//
// Hot buckets (play/peek/audio/summary) are charged on every tap, so they must
// not cost a database write. Implementations are required to fail closed: a
// limiter that cannot decide must return an error, never nil.
type QuizLimiter interface {
	Allow(ctx context.Context, uid int64, bucket string, max int, window time.Duration) error
}

// sharedLimitBuckets stay on the database counter because they are rare
// (a handful of calls per hour) and must be enforced across replicas.
var sharedLimitBuckets = map[string]bool{"start": true, "upload": true}

// LocalLimiter is a per-process fixed-window counter.
//
// It removes the database round-trip on every answer/navigation. The trade-off
// is that the budget is per replica: with N replicas a caller may use up to
// N*max. That is acceptable for anti-abuse buckets whose limits are far above
// human tapping speed; use RedisLimiter when an exact shared cap is required.
type LocalLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	windows map[string]localWindow
}

type localWindow struct {
	start time.Time
	hits  int
}

// NewLocalLimiter returns a limiter with its own clock.
func NewLocalLimiter() *LocalLimiter {
	return &LocalLimiter{now: time.Now, windows: map[string]localWindow{}}
}

// localLimiterSweepAt bounds memory: stale windows are dropped once the map
// grows past this size, so a long-lived process cannot leak per-user entries.
const localLimiterSweepAt = 4096

func (l *LocalLimiter) Allow(_ context.Context, uid int64, bucket string, max int, window time.Duration) error {
	if uid <= 0 || bucket == "" || max <= 0 || window <= 0 {
		return repository.ErrQuizRateLimit
	}
	now := l.now()
	key := fmt.Sprintf("%s:%d", bucket, uid)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.windows == nil {
		l.windows = map[string]localWindow{}
	}
	if len(l.windows) >= localLimiterSweepAt {
		for k, w := range l.windows {
			if !now.Before(w.start.Add(window)) {
				delete(l.windows, k)
			}
		}
	}
	entry, ok := l.windows[key]
	if !ok || !now.Before(entry.start.Add(window)) {
		entry = localWindow{start: now}
	}
	entry.hits++
	l.windows[key] = entry
	if entry.hits > max {
		return repository.ErrQuizRateLimit
	}
	return nil
}

// QuizCounter is the minimal shared counter a RedisLimiter needs.
// It is satisfied by a thin adapter over the project's Redis client.
type QuizCounter interface {
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
}

// RedisLimiter charges a fixed window shared by every replica.
type RedisLimiter struct {
	counter QuizCounter
	now     func() time.Time
}

// NewRedisLimiter returns a limiter backed by a shared counter.
func NewRedisLimiter(counter QuizCounter) *RedisLimiter {
	return &RedisLimiter{counter: counter, now: time.Now}
}

func (l *RedisLimiter) Allow(ctx context.Context, uid int64, bucket string, max int, window time.Duration) error {
	if l == nil || l.counter == nil || uid <= 0 || bucket == "" || max <= 0 || window <= 0 {
		return repository.ErrQuizRateLimit
	}
	now := l.now().UTC()
	slot := now.UnixNano() / int64(window)
	key := fmt.Sprintf("study:quiz:limit:%s:%d:%d", bucket, uid, slot)
	// Keep the key a little past its window so the counter and the slot agree.
	hits, err := l.counter.Incr(ctx, key, window+time.Second)
	if err != nil {
		// Fail closed: an unreachable limiter must not open the endpoint.
		return fmt.Errorf("%w: %v", repository.ErrQuizRateLimit, err)
	}
	if hits > int64(max) {
		return repository.ErrQuizRateLimit
	}
	return nil
}
