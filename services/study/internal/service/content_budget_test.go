package service_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

// budgetLimiter is an atomic in-memory limiter for tests only.
type budgetLimiter struct {
	mu   sync.Mutex
	hits map[string]int
	err  error
}

func (b *budgetLimiter) AllowN(_ context.Context, uid int64, bucket string, n, max int, _ time.Duration) error {
	if b.err != nil {
		return b.err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	k := bucket + string(rune(uid))
	b.hits[k] += n
	if b.hits[k] > max {
		return repository.ErrQuizRateLimit
	}
	return nil
}

// pagedCards returns exactly perPage cards so budget charging is observable
// and records the page size actually requested from the repository.
type pagedCards struct {
	fakeCardRepo
	lastPerPage int
}

func (p *pagedCards) ListByStudySetPaged(_ context.Context, _ int64, _ int, perPage int) ([]model.Flashcard, int, error) {
	p.lastPerPage = perPage
	return make([]model.Flashcard, perPage), 1000, nil
}

func contentSvc(l service.ContentLimiter, pol service.ContentPolicy) (*service.StudySetService, *fakeSetRepo, *pagedCards) {
	repo := newFakeSetRepo()
	cards := &pagedCards{}
	return service.NewStudySetService(repo, cards).WithLimiter(l, pol), repo, cards
}

func TestNonOwnerPageSizeIsCapped(t *testing.T) {
	svc, repo, cards := contentSvc(nil, service.DefaultContentPolicy())
	set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Pub", Visibility: "public"})
	res, err := svc.GetFlashcardsPaged(context.Background(), set.ID, 2, model.FlashcardFilter{Page: 1, PerPage: 999999})
	if err != nil {
		t.Fatal(err)
	}
	if cards.lastPerPage != 20 || len(res.Items) != 20 {
		t.Fatalf("non-owner page not capped: %d", cards.lastPerPage)
	}
	if _, err = svc.GetFlashcardsPaged(context.Background(), set.ID, 1, model.FlashcardFilter{Page: 1, PerPage: 200}); err != nil || cards.lastPerPage != 200 {
		t.Fatalf("owner must keep editor page size, got %d err=%v", cards.lastPerPage, err)
	}
}

func TestContentBudgetIsSharedAcrossEndpoints(t *testing.T) {
	pol := service.ContentPolicy{MaxPageSize: 20, RequestsPerMin: 100, CardsPerWindow: 60, CardWindow: time.Minute, EnforceLimits: true}
	svc, repo, _ := contentSvc(&budgetLimiter{hits: map[string]int{}}, pol)
	set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Pub", Visibility: "public"})
	ctx := context.Background()
	// 20 (detail) + 20 + 20 = 60 allowed, next request (any endpoint) is rejected.
	if _, err := svc.GetWithCards(ctx, set.ID, 2); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := svc.GetFlashcardsPaged(ctx, set.ID, 2, model.FlashcardFilter{PerPage: 20}); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	if _, err := svc.GetWithCards(ctx, set.ID, 2); !errors.Is(err, repository.ErrQuizRateLimit) {
		t.Fatalf("detail should share budget, got %v", err)
	}
	if _, err := svc.GetFlashcardsPaged(ctx, set.ID, 2, model.FlashcardFilter{PerPage: 20}); !errors.Is(err, repository.ErrQuizRateLimit) {
		t.Fatalf("paged should share budget, got %v", err)
	}
	// Another account is unaffected; owner is exempt.
	if _, err := svc.GetFlashcardsPaged(ctx, set.ID, 3, model.FlashcardFilter{PerPage: 20}); err != nil {
		t.Fatalf("other account blocked: %v", err)
	}
	if _, err := svc.GetFlashcardsPaged(ctx, set.ID, 1, model.FlashcardFilter{PerPage: 20}); err != nil {
		t.Fatalf("owner blocked: %v", err)
	}
}

func TestObserveOnlyModeAndLimiterFailure(t *testing.T) {
	pol := service.ContentPolicy{MaxPageSize: 20, RequestsPerMin: 1, CardsPerWindow: 1, CardWindow: time.Minute, EnforceLimits: false}
	svc, repo, _ := contentSvc(&budgetLimiter{hits: map[string]int{}}, pol)
	set, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Pub", Visibility: "public"})
	for i := 0; i < 3; i++ {
		if _, err := svc.GetFlashcardsPaged(context.Background(), set.ID, 2, model.FlashcardFilter{PerPage: 20}); err != nil {
			t.Fatalf("observe-only must not block: %v", err)
		}
	}
	// Limiter outage never opens access: unauthorized/private checks run first.
	svc, repo, _ = contentSvc(&budgetLimiter{hits: map[string]int{}, err: errors.New("db down")}, service.DefaultContentPolicy())
	priv, _ := repo.Create(context.Background(), 1, model.CreateStudySetInput{Title: "Priv", Visibility: "private"})
	if _, err := svc.GetFlashcardsPaged(context.Background(), priv.ID, 2, model.FlashcardFilter{}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("private set must stay forbidden, got %v", err)
	}
}
