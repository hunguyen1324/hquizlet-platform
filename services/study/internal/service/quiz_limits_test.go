package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

type denyLimiter struct{}

func (denyLimiter) Allow(context.Context, int64, string, int, time.Duration) error {
	return repository.ErrQuizRateLimit
}

type fakeCounter struct {
	hits  int64
	ttl   time.Duration
	calls int
	err   error
}

func (f *fakeCounter) Incr(_ context.Context, _ string, ttl time.Duration) (int64, error) {
	f.calls++
	f.ttl = ttl
	if f.err != nil {
		return 0, f.err
	}
	f.hits++
	return f.hits, nil
}

func TestLocalLimiterEnforcesWindowPerUserAndBucket(t *testing.T) {
	limiter := NewLocalLimiter()
	now := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := limiter.Allow(ctx, 7, "play", 3, time.Minute); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if err := limiter.Allow(ctx, 7, "play", 3, time.Minute); !errors.Is(err, repository.ErrQuizRateLimit) {
		t.Fatalf("over budget accepted: %v", err)
	}
	if err := limiter.Allow(ctx, 8, "play", 3, time.Minute); err != nil {
		t.Fatalf("other user blocked: %v", err)
	}
	if err := limiter.Allow(ctx, 7, "peek", 3, time.Minute); err != nil {
		t.Fatalf("other bucket blocked: %v", err)
	}
	now = now.Add(time.Minute)
	if err := limiter.Allow(ctx, 7, "play", 3, time.Minute); err != nil {
		t.Fatalf("window did not reset: %v", err)
	}
}

func TestRedisLimiterSharesCounterAndFailsClosed(t *testing.T) {
	counter := &fakeCounter{}
	limiter := NewRedisLimiter(counter)
	limiter.now = func() time.Time { return time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	if err := limiter.Allow(ctx, 7, "play", 1, time.Minute); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if counter.ttl < time.Minute {
		t.Fatalf("counter key expires too early: %s", counter.ttl)
	}
	if err := limiter.Allow(ctx, 7, "play", 1, time.Minute); !errors.Is(err, repository.ErrQuizRateLimit) {
		t.Fatalf("over budget accepted: %v", err)
	}
	counter.err = errors.New("redis down")
	if err := limiter.Allow(ctx, 9, "play", 1, time.Minute); !errors.Is(err, repository.ErrQuizRateLimit) {
		t.Fatalf("limiter did not fail closed: %v", err)
	}
}

func TestHotQuizBucketsSkipDatabaseCounter(t *testing.T) {
	svc, _, store, _ := securityFixture(t)
	ctx := context.Background()
	if err := svc.Allow(ctx, 10, "play", 2, time.Minute); err != nil {
		t.Fatal(err)
	}
	if store.allowHits != 0 {
		t.Fatalf("hot bucket wrote the database counter %d times", store.allowHits)
	}
	// Session creation stays on the shared counter: it must hold across replicas.
	if err := svc.Allow(ctx, 10, "start", 20, time.Hour); err != nil {
		t.Fatal(err)
	}
	if store.allowHits != 1 {
		t.Fatalf("start should use the shared counter, hits=%d", store.allowHits)
	}
}

func TestQuizPeekIsReadOnlyAndGrantsNothing(t *testing.T) {
	svc, _, store, _ := securityFixture(t)
	ctx := context.Background()
	view := startSecurityQuiz(t, svc, false)
	peek, err := svc.Peek(ctx, 1, 10, view.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(peek.Page) != 1 || peek.Page[0].Index != 1 {
		t.Fatalf("unexpected peek page: %+v", peek.Page)
	}
	data, _ := json.Marshal(peek)
	for _, secret := range []string{"correctAnswer", "answerExplanation", "audioUrl", "SECRET_EXPLANATION", `"isCorrect":true`, "origin.example"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("peek leaked %s: %s", secret, data)
		}
	}
	if !strings.Contains(string(data), "UNSEEN_QUESTION") {
		t.Fatalf("peek did not return page content: %s", data)
	}
	// A prefetched page must not become answerable, audible or resumable.
	if _, err := svc.Act(ctx, 1, 10, view.ID, "answer", 1, "right"); !errors.Is(err, ErrValidation) {
		t.Fatalf("peek granted answer rights: %v", err)
	}
	if _, err := svc.Audio(ctx, 1, 10, view.ID, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("peek granted audio rights: %v", err)
	}
	if stored := store.state(view.ID); stored.Seen[1] || stored.CurrentIndex != 0 {
		t.Fatalf("peek mutated the stored session: seen=%v current=%d", stored.Seen, stored.CurrentIndex)
	}
	// Another user's session stays invisible.
	if _, err := svc.Peek(ctx, 1, 20, view.ID, 0); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("cross-user peek: %v", err)
	}
	// Peek cannot be used to enumerate the whole quiz page by page.
	if _, err := svc.Peek(ctx, 1, 10, view.ID, 0+peekLookAhead+1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("peek outside the prefetch window: %v", err)
	}
}

func TestQuizSessionSanitizesHTMLAtCreation(t *testing.T) {
	sets := &protectedSets{set: model.StudySet{ID: 1, UserID: 10, ContentType: "quiz", Visibility: "public"}}
	answer := "b"
	explanation := `<p onclick="steal()">keep<strong>this</strong></p>`
	questions := &protectedQuestions{items: []model.QuizQuestion{{
		QuestionType:      "multiple_choice",
		QuestionText:      `<script>alert(1)</script>Read <strong>this</strong>`,
		CorrectAnswer:     &answer,
		AnswerExplanation: &explanation,
		Options:           []model.QuizQuestionOption{{Text: "a"}, {Text: "b", IsCorrect: true}},
	}}}
	store := &memoryQuizSessions{states: map[string][]byte{}, now: time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)}
	payment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"hasAccess":true}`))
	}))
	defer payment.Close()
	snapshots := newMemoryQuizSnapshots()
	svc := NewProtectedQuizService(sets, questions, store, snapshots, payment.URL)
	svc.now = func() time.Time { return store.now }
	view, err := svc.Start(context.Background(), 1, 10, model.StartQuizInput{Mode: "practice", Layout: "default"})
	if err != nil {
		t.Fatal(err)
	}
	stored := store.state(view.ID)
	if len(stored.Items) != 0 || stored.SnapshotID == "" {
		t.Fatalf("session kept a private copy of the questions: items=%d snapshot=%q", len(stored.Items), stored.SnapshotID)
	}
	snap := snapshots.snaps[stored.SnapshotID]
	if strings.Contains(snap.Items[0].Question.QuestionText, "<script") {
		t.Fatalf("question text not sanitized: %s", snap.Items[0].Question.QuestionText)
	}
	cleanExplanation := snap.Items[0].Question.AnswerExplanation
	if cleanExplanation == nil || strings.Contains(*cleanExplanation, "onclick") {
		t.Fatalf("explanation not sanitized: %v", cleanExplanation)
	}
	if !strings.Contains(*cleanExplanation, "<strong>this</strong>") {
		t.Fatalf("sanitizer dropped safe formatting: %s", *cleanExplanation)
	}
	// The response is built from the snapshot, so the learner sees the same HTML
	// the sanitizer produced rather than the raw rows.
	if strings.Contains(view.Page[0].Question.QuestionText, "<script") {
		t.Fatal("unsanitized page served")
	}
}

func TestLegacySessionIsStillSanitizedOnRead(t *testing.T) {
	svc, _, _, _ := securityFixture(t)
	// A session with no snapshot and no Sanitized flag predates sanitize-once
	// and still stores its own raw questions in state.
	svc.snapshots = nil
	view := startSecurityQuiz(t, svc, false)
	if err := svc.sessions.Update(context.Background(), view.ID, 10, 1, func(s *model.QuizSession) error {
		s.Sanitized = false
		s.Items[0].Question.QuestionText = `<script>alert(1)</script>safe`
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	paged, err := svc.Act(context.Background(), 1, 10, view.ID, "page", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	text := paged.Page[0].Question.QuestionText
	if strings.Contains(text, "<script") || !strings.Contains(text, "safe") {
		t.Fatalf("legacy page was served unsanitized: %s", text)
	}
}
