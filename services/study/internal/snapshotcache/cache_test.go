package snapshotcache

import (
	"context"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

type countingRepo struct {
	snaps map[string]model.QuizSnapshot
	gets  int
	puts  int
}

func (r *countingRepo) Get(_ context.Context, id string) (model.QuizSnapshot, error) {
	r.gets++
	snap, ok := r.snaps[id]
	if !ok {
		return model.QuizSnapshot{}, repository.ErrNotFound
	}
	return snap, nil
}

func (r *countingRepo) Put(_ context.Context, snap model.QuizSnapshot) error {
	r.puts++
	r.snaps[snap.ID] = snap
	return nil
}

func (r *countingRepo) DeleteUnreferenced(context.Context, time.Time) (int64, error) { return 0, nil }

func snapshot(id string, questions int) model.QuizSnapshot {
	items := make([]model.SessionItem, questions)
	for i := range items {
		items[i] = model.SessionItem{Number: i + 1, Question: model.QuizQuestion{QuestionText: "question text for one item"}}
	}
	return model.QuizSnapshot{ID: id, Items: items, CreatedAt: time.Now()}
}

func TestCacheServesRepeatedReadsFromMemory(t *testing.T) {
	repo := &countingRepo{snaps: map[string]model.QuizSnapshot{"a": snapshot("a", 3)}}
	cache := New(repo, DefaultBytes)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		snap, err := cache.Get(ctx, "a")
		if err != nil || len(snap.Items) != 3 {
			t.Fatalf("get %d: %v", i, err)
		}
	}
	if repo.gets != 1 {
		t.Fatalf("expected one database read, got %d", repo.gets)
	}
	hits, misses, _ := cache.Stats()
	if hits != 4 || misses != 1 {
		t.Fatalf("hits=%d misses=%d", hits, misses)
	}
}

func TestCacheEvictsLeastRecentlyUsedWithinBudget(t *testing.T) {
	one := sizeOf(snapshot("a", 1))
	repo := &countingRepo{snaps: map[string]model.QuizSnapshot{}}
	cache := New(repo, one*2) // room for two entries
	ctx := context.Background()
	for _, id := range []string{"a", "b", "c"} {
		if err := cache.Put(ctx, snapshot(id, 1)); err != nil {
			t.Fatal(err)
		}
	}
	// "a" is the oldest entry, so it was evicted and must be read again.
	before := repo.gets
	if _, err := cache.Get(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	if repo.gets != before+1 {
		t.Fatal("expected the evicted snapshot to be re-read")
	}
	if _, _, used := cache.Stats(); used > one*2 {
		t.Fatalf("cache exceeded its budget: %d > %d", used, one*2)
	}
	// "c" is still cached.
	before = repo.gets
	if _, err := cache.Get(ctx, "c"); err != nil {
		t.Fatal(err)
	}
	if repo.gets != before {
		t.Fatal("recent entry was evicted")
	}
}

func TestCacheReturnsNotFoundWithoutCachingIt(t *testing.T) {
	repo := &countingRepo{snaps: map[string]model.QuizSnapshot{}}
	cache := New(repo, DefaultBytes)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if _, err := cache.Get(ctx, "missing"); err != repository.ErrNotFound {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	}
	if repo.gets != 2 {
		t.Fatalf("missing snapshots must not be cached: %d reads", repo.gets)
	}
}
