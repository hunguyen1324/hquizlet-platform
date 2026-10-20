package service

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

type protectedSets struct {
	repository.StudySets
	set model.StudySet
}

func (f *protectedSets) Get(context.Context, int64) (model.StudySet, error) { return f.set, nil }
func (f *protectedSets) IsOwner(_ context.Context, _ int64, uid int64) (bool, error) {
	return f.set.UserID == uid, nil
}

type protectedQuestions struct {
	repository.QuizQuestions
	items []model.QuizQuestion
}

func (f *protectedQuestions) ListByStudySet(context.Context, int64) ([]model.QuizQuestion, error) {
	return f.items, nil
}

type memoryQuizSessions struct {
	mu        sync.Mutex
	states    map[string][]byte
	allowHits int
	now       time.Time
}

func (f *memoryQuizSessions) Create(_ context.Context, s model.QuizSession) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, _ := json.Marshal(s)
	f.states[s.ID] = data
	return nil
}
func (f *memoryQuizSessions) Allow(context.Context, int64, string, int, time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.allowHits++
	return nil
}
func (f *memoryQuizSessions) Recent(context.Context, int64, int64) ([]string, error) {
	return []string{}, nil
}
func (f *memoryQuizSessions) CountActive(_ context.Context, _ int64, _ int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.states), nil
}
func (f *memoryQuizSessions) List(_ context.Context, _ int64, _ int64, _, _ int) ([]repository.QuizSessionMeta, int, error) {
	return []repository.QuizSessionMeta{}, 0, nil
}
func (f *memoryQuizSessions) Delete(_ context.Context, id string, _ int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.states[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.states, id)
	return nil
}
func (f *memoryQuizSessions) View(_ context.Context, id string, uid, setID int64, fn func(*model.QuizSession) error) error {
	f.mu.Lock()
	data, ok := f.states[id]
	f.mu.Unlock()
	if !ok {
		return repository.ErrNotFound
	}
	var s model.QuizSession
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s.UserID != uid || s.StudySetID != setID || !s.ExpiresAt.After(f.now) {
		return repository.ErrNotFound
	}
	// The callback receives a copy: nothing it does may be persisted.
	return fn(&s)
}
func (f *memoryQuizSessions) state(id string) model.QuizSession {
	f.mu.Lock()
	defer f.mu.Unlock()
	var s model.QuizSession
	json.Unmarshal(f.states[id], &s)
	return s
}
func (f *memoryQuizSessions) Update(_ context.Context, id string, uid, setID int64, fn func(*model.QuizSession) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.states[id]
	if !ok {
		return repository.ErrNotFound
	}
	var s model.QuizSession
	json.Unmarshal(data, &s)
	if s.UserID != uid || s.StudySetID != setID || !s.ExpiresAt.After(f.now) {
		return repository.ErrNotFound
	}
	if err := fn(&s); err != nil {
		return err
	}
	f.states[id], _ = json.Marshal(s)
	return nil
}

type memoryQuizSnapshots struct {
	mu    sync.Mutex
	snaps map[string]model.QuizSnapshot
	gets  int
	puts  int
}

func newMemoryQuizSnapshots() *memoryQuizSnapshots {
	return &memoryQuizSnapshots{snaps: map[string]model.QuizSnapshot{}}
}

func (f *memoryQuizSnapshots) Put(_ context.Context, snap model.QuizSnapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.puts++
	if _, ok := f.snaps[snap.ID]; ok {
		return nil
	}
	f.snaps[snap.ID] = snap
	return nil
}

func (f *memoryQuizSnapshots) Get(_ context.Context, id string) (model.QuizSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets++
	snap, ok := f.snaps[id]
	if !ok {
		return model.QuizSnapshot{}, repository.ErrNotFound
	}
	return snap, nil
}

func (f *memoryQuizSnapshots) DeleteUnreferenced(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func securityFixture(t *testing.T) (*ProtectedQuizService, *protectedSets, *memoryQuizSessions, *memoryQuizSnapshots) {
	t.Helper()
	snapshots := newMemoryQuizSnapshots()
	t.Helper()
	correct := "B"
	explanation := "SECRET_EXPLANATION"
	audio := "https://origin.example/secret.mp3"
	sets := &protectedSets{set: model.StudySet{ID: 1, UserID: 10, ContentType: "quiz", Visibility: "public"}}
	questions := &protectedQuestions{items: []model.QuizQuestion{
		{ID: 1, QuestionType: "multiple_choice", QuestionText: "First", CorrectAnswer: &correct, AnswerExplanation: &explanation, AudioURL: &audio, Options: []model.QuizQuestionOption{{Text: "wrong"}, {Text: "right", Position: 1, IsCorrect: true}}},
		{ID: 2, QuestionType: "multiple_choice", QuestionText: "UNSEEN_QUESTION", CorrectAnswer: &correct, AudioURL: &audio, Options: []model.QuizQuestionOption{{Text: "wrong"}, {Text: "right", Position: 1}}},
	}}
	now := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	store := &memoryQuizSessions{states: map[string][]byte{}, now: now}
	payment := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"hasAccess":true}`)) }))
	t.Cleanup(payment.Close)
	svc := NewProtectedQuizService(sets, questions, store, snapshots, payment.URL)
	svc.now = func() time.Time { return store.now }
	return svc, sets, store, snapshots
}
func startSecurityQuiz(t *testing.T, s *ProtectedQuizService, instant bool) model.QuizSessionView {
	t.Helper()
	view, err := s.Start(context.Background(), 1, 10, model.StartQuizInput{Mode: "practice", Layout: "default", Instant: instant, DurationSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	return view
}
func TestQuizPageDoesNotLeakSolutionsOrOrigin(t *testing.T) {
	svc, _, _, _ := securityFixture(t)
	view := startSecurityQuiz(t, svc, false)
	data, _ := json.Marshal(view)
	for _, secret := range []string{"correctAnswer", "answerExplanation", "audioUrl", "subQuestions", `"isCorrect":true`, "SECRET_EXPLANATION", "origin.example", "UNSEEN_QUESTION"} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("leaked %s: %s", secret, data)
		}
	}
	if len(view.Manifest) != 2 || len(view.Page) != 1 || !view.Page[0].HasAudio {
		t.Fatalf("unexpected page: %+v", view)
	}
}
func TestQuizOwnershipAccessAndLegacyExport(t *testing.T) {
	svc, sets, _, _ := securityFixture(t)
	ctx := context.Background()
	view := startSecurityQuiz(t, svc, false)
	if _, err := svc.Act(ctx, 1, 20, view.ID, "page", 0, ""); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("other account accessed session: %v", err)
	}
	if _, err := svc.Act(ctx, 2, 10, view.ID, "page", 0, ""); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("other set accessed session: %v", err)
	}
	if _, err := svc.Summary(ctx, 1, 0); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("anonymous access: %v", err)
	}
	sets.set.Visibility = "private"
	if _, err := svc.Summary(ctx, 1, 20); !errors.Is(err, ErrForbidden) {
		t.Fatalf("private access: %v", err)
	}
	legacy := NewQuizQuestionService(svc.questions, sets)
	if _, err := legacy.ListByStudySet(ctx, 1, 20); !errors.Is(err, ErrForbidden) {
		t.Fatalf("legacy answer dump allowed: %v", err)
	}
	if _, err := legacy.ListByStudySet(ctx, 1, 10); err != nil {
		t.Fatal(err)
	}
}
func TestQuizInstantAnswerLocksAndDeferredDoesNotReveal(t *testing.T) {
	svc, _, _, _ := securityFixture(t)
	ctx := context.Background()
	view := startSecurityQuiz(t, svc, true)
	answered, err := svc.Act(ctx, 1, 10, view.ID, "answer", 0, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if !answered.Page[0].Revealed || answered.Results[0] {
		t.Fatalf("wrong instant result: %+v", answered)
	}
	answered, err = svc.Act(ctx, 1, 10, view.ID, "answer", 0, "right")
	if err != nil {
		t.Fatal(err)
	}
	if answered.Answers[0] != "wrong" {
		t.Fatal("instant answer changed after solution was revealed")
	}
	if _, err = svc.Act(ctx, 1, 10, view.ID, "answer", 1, "right"); !errors.Is(err, ErrValidation) {
		t.Fatal("accepted answer to unseen question")
	}
	deferred := startSecurityQuiz(t, svc, false)
	answered, err = svc.Act(ctx, 1, 10, deferred.ID, "answer", 0, "right")
	if err != nil {
		t.Fatal(err)
	}
	if answered.Page[0].Question.CorrectAnswer != nil || len(answered.Results) != 0 {
		t.Fatal("deferred answer leaked result")
	}
	submitted, err := svc.Act(ctx, 1, 10, deferred.ID, "submit", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Score == nil || *submitted.Score != 1 || !submitted.Page[0].Revealed {
		t.Fatalf("bad server grade: %+v", submitted)
	}
	blank, err := svc.Act(ctx, 1, 10, deferred.ID, "page", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if blank.Page[0].Revealed || blank.Page[0].Question.CorrectAnswer != nil {
		t.Fatal("blank submit dumped answer")
	}
}
func TestQuizDeadlineCannotBeExtendedAndSubmissionIsIdempotent(t *testing.T) {
	svc, _, store, _ := securityFixture(t)
	view := startSecurityQuiz(t, svc, false)
	store.now = store.now.Add(61 * time.Second)
	result, err := svc.Act(context.Background(), 1, 10, view.ID, "answer", 0, "right")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Submitted || len(result.Answers) != 0 || result.TimeUsed != 60 || *result.Score != 0 {
		t.Fatalf("late answer accepted: %+v", result)
	}
	result2, err := svc.Act(context.Background(), 1, 10, view.ID, "submit", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if result2.TimeUsed != result.TimeUsed || *result2.Score != *result.Score {
		t.Fatal("submission changed")
	}
	store.now = store.now.Add(25 * time.Hour)
	if _, err = svc.Act(context.Background(), 1, 10, view.ID, "page", 0, ""); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal("expired session replayed")
	}
}
func TestQuizAudioRequiresOwnDeliveredQuestionAndRateLimit(t *testing.T) {
	svc, _, _, _ := securityFixture(t)
	view := startSecurityQuiz(t, svc, false)
	ctx := context.Background()
	if _, err := svc.Audio(ctx, 1, 10, view.ID, 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("unseen audio allowed")
	}
	if _, err := svc.Audio(ctx, 1, 20, view.ID, 0); !errors.Is(err, repository.ErrNotFound) {
		t.Fatal("stolen session allowed audio")
	}
	if source, err := svc.Audio(ctx, 1, 10, view.ID, 0); err != nil || source == "" {
		t.Fatalf("legitimate audio denied: %v", err)
	}
	svc.WithLimiter(denyLimiter{})
	if _, err := svc.Audio(ctx, 1, 10, view.ID, 0); !errors.Is(err, repository.ErrQuizRateLimit) {
		t.Fatal("audio bypassed rate limit")
	}
}
func TestQuizAccessRevocationAndPaymentFailClosed(t *testing.T) {
	svc, sets, _, _ := securityFixture(t)
	ctx := context.Background()
	view, err := svc.Start(ctx, 1, 20, model.StartQuizInput{Mode: "practice", Layout: "default"})
	if err != nil {
		t.Fatal(err)
	}
	sets.set.Visibility = "private"
	if _, err = svc.Act(ctx, 1, 20, view.ID, "page", 0, ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked visibility ignored")
	}
	sets.set.Visibility = "public"
	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"hasAccess":false}`)) }))
	defer denied.Close()
	svc.paymentURL = denied.URL
	if _, err = svc.Act(ctx, 1, 20, view.ID, "page", 0, ""); !errors.Is(err, ErrForbidden) {
		t.Fatal("revoked entitlement ignored")
	}
	denied.Close()
	if _, err = svc.Act(ctx, 1, 20, view.ID, "page", 0, ""); err == nil {
		t.Fatal("payment failure opened access")
	}
}
func TestQuizHTMLDoesNotShipEmbeddedURLs(t *testing.T) {
	raw := `<p onclick="steal()">Read <strong>this</strong><audio src="https://secret.example/a.mp3">secret</audio><script>alert(1)</script><img src="https://secret.example/image" alt="Picture"></p>`
	clean := safeQuizHTML(raw)
	for _, bad := range []string{"onclick", "secret.example", "<script", "<audio", "alert(1)"} {
		if strings.Contains(clean, bad) {
			t.Fatalf("unsafe HTML: %s", clean)
		}
	}
	if !strings.Contains(clean, "<strong>this</strong>") {
		t.Fatalf("formatting lost: %s", clean)
	}
}

func TestSessionItemsInheritParagraphImage(t *testing.T) {
	image := "/images/passage.png"
	items, err := sessionItems([]model.QuizQuestion{{QuestionType: "paragraph", ImageURL: &image, SubQuestions: json.RawMessage("[{\"questionText\":\"Q\",\"questionType\":\"written\"}]")}}, true)
	if err != nil || len(items) != 1 {
		t.Fatalf("items: %v, %v", items, err)
	}
	if items[0].Question.ImageURL == nil || *items[0].Question.ImageURL != image {
		t.Fatal("parent image was lost")
	}
}

func TestQuizSessionsShareOneSnapshotAndKeepStateSmall(t *testing.T) {
	svc, _, store, snapshots := securityFixture(t)
	ctx := context.Background()
	first := startSecurityQuiz(t, svc, false)
	second := startSecurityQuiz(t, svc, false)
	a, b := store.state(first.ID), store.state(second.ID)
	if a.SnapshotID == "" || a.SnapshotID != b.SnapshotID {
		t.Fatalf("sessions did not share a snapshot: %q %q", a.SnapshotID, b.SnapshotID)
	}
	if len(a.Items) != 0 || len(b.Items) != 0 {
		t.Fatal("session state still holds a private copy of the questions")
	}
	if a.Total != 2 {
		t.Fatalf("session total = %d, want 2", a.Total)
	}
	if _, err := svc.Act(ctx, 1, 10, first.ID, "page", 0, ""); err != nil {
		t.Fatal(err)
	}
	if snapshots.gets == 0 {
		t.Fatal("page serving never resolved the snapshot")
	}
	if len(snapshots.snaps) != 1 {
		t.Fatalf("identical content stored %d snapshots", len(snapshots.snaps))
	}
	// The mutable state must not carry question content or solutions.
	state, _ := json.Marshal(a)
	for _, secret := range []string{"UNSEEN_QUESTION", "correctAnswer", "SECRET_EXPLANATION", "origin.example"} {
		if strings.Contains(string(state), secret) {
			t.Fatalf("state leaked %s: %s", secret, state)
		}
	}
}

func TestQuizPageResponseIsCompactAndManifestIsOptIn(t *testing.T) {
	svc, _, _, _ := securityFixture(t)
	ctx := context.Background()
	started := startSecurityQuiz(t, svc, false)
	if len(started.Manifest) != 2 || started.Total != 2 {
		t.Fatalf("start payload: %+v", started)
	}
	startJSON, _ := json.Marshal(started)
	if _, err := svc.Act(ctx, 1, 10, started.ID, "answer", 0, "right"); err != nil {
		t.Fatal(err)
	}
	paged, err := svc.Act(ctx, 1, 10, started.ID, "page", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(paged.Manifest) != 0 {
		t.Fatalf("page carried the manifest: %d entries", len(paged.Manifest))
	}
	if paged.Total != 2 || paged.Answered != 1 || len(paged.Answers) != 0 || len(paged.Results) != 0 {
		t.Fatalf("page payload is not page-scoped: %+v", paged)
	}
	if len(paged.Page) != 1 || paged.Page[0].Index != 1 {
		t.Fatalf("wrong page: %+v", paged.Page)
	}
	pageJSON, _ := json.Marshal(paged)
	if len(pageJSON) >= len(startJSON) {
		t.Fatalf("page response is not smaller than start: %d >= %d", len(pageJSON), len(startJSON))
	}
	// Resuming in a fresh tab asks for the manifest once.
	resumed, err := svc.ActWith(ctx, 1, 10, started.ID, "page", 1, "", ActOptions{IncludeManifest: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed.Manifest) != 2 {
		t.Fatalf("resume did not include the manifest: %+v", resumed)
	}
	// Instant answers return only that page's result, not the whole grading map.
	instant := startSecurityQuiz(t, svc, true)
	answered, err := svc.Act(ctx, 1, 10, instant.ID, "answer", 0, "wrong")
	if err != nil {
		t.Fatal(err)
	}
	if answered.Results[0] || answered.Answers[0] != "wrong" || len(answered.Manifest) != 0 {
		t.Fatalf("instant answer payload: %+v", answered)
	}
}
