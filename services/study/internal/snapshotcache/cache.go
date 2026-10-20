// Package snapshotcache keeps immutable quiz snapshots in process memory.
//
// A snapshot is content-addressed and never changes, so a replica may cache it
// indefinitely. The cache is bounded by an estimate of the decoded size, not by
// an entry count: one TOEIC quiz is a few hundred KB while a tiny quiz is a few
// KB, and the byte budget is what protects the process.
package snapshotcache

import (
	"container/list"
	"context"
	"sync"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// DefaultBytes is the per-replica budget. A 200-question TOEIC snapshot is well
// under 1 MB, so this holds hundreds of distinct quizzes.
const DefaultBytes = 64 << 20

type Cache struct {
	mu      sync.Mutex
	repo    repository.QuizSnapshots
	budget  int64
	used    int64
	lru     *list.List // front = most recently used
	entries map[string]*list.Element
	hits    int64
	misses  int64
}

type entry struct {
	id    string
	snap  model.QuizSnapshot
	bytes int64
}

func New(repo repository.QuizSnapshots, budget int64) *Cache {
	if budget <= 0 {
		budget = DefaultBytes
	}
	return &Cache{repo: repo, budget: budget, lru: list.New(), entries: map[string]*list.Element{}}
}

func (c *Cache) Get(ctx context.Context, id string) (model.QuizSnapshot, error) {
	c.mu.Lock()
	if el, ok := c.entries[id]; ok {
		c.lru.MoveToFront(el)
		c.hits++
		snap := el.Value.(*entry).snap
		c.mu.Unlock()
		return snap, nil
	}
	c.misses++
	c.mu.Unlock()

	snap, err := c.repo.Get(ctx, id)
	if err != nil {
		return model.QuizSnapshot{}, err
	}
	c.store(snap)
	return snap, nil
}

func (c *Cache) Put(ctx context.Context, snap model.QuizSnapshot) error {
	if err := c.repo.Put(ctx, snap); err != nil {
		return err
	}
	c.store(snap)
	return nil
}

func (c *Cache) DeleteUnreferenced(ctx context.Context, before time.Time) (int64, error) {
	// Cached entries stay: a snapshot is immutable and content-addressed, so an
	// entry whose row was just removed is still the right answer for any session
	// that pointed at it. Dropping the cache here would only cause cold reads.
	return c.repo.DeleteUnreferenced(ctx, before)
}

// Stats reports cache counters for observability and tests.
func (c *Cache) Stats() (hits, misses, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses, c.used
}

func (c *Cache) store(snap model.QuizSnapshot) {
	size := sizeOf(snap)
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[snap.ID]; ok {
		c.lru.MoveToFront(el)
		return
	}
	el := c.lru.PushFront(&entry{id: snap.ID, snap: snap, bytes: size})
	c.entries[snap.ID] = el
	c.used += size
	for c.used > c.budget && c.lru.Len() > 1 {
		back := c.lru.Back()
		if back == nil {
			break
		}
		victim := back.Value.(*entry)
		c.lru.Remove(back)
		delete(c.entries, victim.id)
		c.used -= victim.bytes
	}
}

// sizeOf estimates the decoded footprint. Go strings are byte slices, so the
// byte length is the stored size; the constants cover slice/struct overhead.
func sizeOf(snap model.QuizSnapshot) int64 {
	n := int64(512 + len(snap.ID))
	for _, it := range snap.Items {
		q := it.Question
		n += 320 + int64(len(q.QuestionText)+len(q.QuestionType))
		for _, ptr := range []*string{q.CorrectAnswer, q.AnswerExplanation, q.ParagraphText, q.ImageURL, q.AudioURL} {
			if ptr != nil {
				n += int64(len(*ptr))
			}
		}
		n += int64(len(q.Tags) * 24)
		for _, o := range q.Options {
			n += 80 + int64(len(o.Text))
		}
	}
	n += int64(len(snap.Manifest)) * 32
	for _, page := range snap.Pages {
		n += 24 + int64(len(page))*8
	}
	return n
}
