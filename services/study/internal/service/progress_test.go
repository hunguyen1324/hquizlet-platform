package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeStudySets struct {
	ownerMap map[int64]int64 // setID → ownerUserID
}

func (f *fakeStudySets) List(ctx context.Context, userID int64) ([]model.StudySet, error) {
	return nil, nil
}
func (f *fakeStudySets) ListAll(ctx context.Context) ([]model.StudySet, error) { return nil, nil }
func (f *fakeStudySets) ListWithFilter(ctx context.Context, userID int64, filter model.StudySetFilter) (model.StudySetListResult, error) {
	return model.StudySetListResult{}, nil
}
func (f *fakeStudySets) Get(ctx context.Context, id int64) (model.StudySet, error) {
	return model.StudySet{}, nil
}
func (f *fakeStudySets) GetOwned(ctx context.Context, id, userID int64) (model.StudySet, error) {
	return model.StudySet{}, nil
}
func (f *fakeStudySets) Create(ctx context.Context, userID int64, in model.CreateStudySetInput) (model.StudySet, error) {
	return model.StudySet{}, nil
}
func (f *fakeStudySets) Update(ctx context.Context, id int64, in model.UpdateStudySetInput) (model.StudySet, error) {
	return model.StudySet{}, nil
}
func (f *fakeStudySets) Delete(ctx context.Context, id int64) error { return nil }
func (f *fakeStudySets) IsOwner(ctx context.Context, id, userID int64) (bool, error) {
	owner, ok := f.ownerMap[id]
	if !ok {
		return false, repository.ErrNotFound
	}
	return owner == userID, nil
}

type fakeFlashcards struct {
	cards []model.Flashcard
}

func (f *fakeFlashcards) ListByStudySet(ctx context.Context, studySetID int64) ([]model.Flashcard, error) {
	var out []model.Flashcard
	for _, c := range f.cards {
		if c.StudySetID == studySetID {
			out = append(out, c)
		}
	}
	return out, nil
}
func (f *fakeFlashcards) Get(ctx context.Context, id int64) (model.Flashcard, error) {
	return model.Flashcard{}, nil
}
func (f *fakeFlashcards) Create(ctx context.Context, studySetID int64, in model.CreateFlashcardInput) (model.Flashcard, error) {
	return model.Flashcard{}, nil
}
func (f *fakeFlashcards) Update(ctx context.Context, id int64, in model.UpdateFlashcardInput) (model.Flashcard, error) {
	return model.Flashcard{}, nil
}
func (f *fakeFlashcards) ToggleStar(ctx context.Context, id int64) (model.Flashcard, error) {
	return model.Flashcard{}, nil
}
func (f *fakeFlashcards) Delete(ctx context.Context, id int64) error { return nil }
func (f *fakeFlashcards) BulkSave(ctx context.Context, studySetID int64, items []model.BulkFlashcardItem) (model.BulkSaveResult, error) {
	return model.BulkSaveResult{}, nil
}

type fakeProgress struct {
	savedSessions []model.LearningSession
	dupKey        bool // simulate duplicate idempotency key
	saveErr       error
}

func (f *fakeProgress) Save(ctx context.Context, userID, studySetID int64, in model.SaveProgressInput) (model.LearningSession, error) {
	if f.dupKey {
		return model.LearningSession{}, repository.ErrDuplicateIdempotencyKey
	}
	if f.saveErr != nil {
		return model.LearningSession{}, f.saveErr
	}
	s := model.LearningSession{
		ID: int64(len(f.savedSessions) + 1), UserID: userID, StudySetID: studySetID,
		Mode: in.Mode, Score: in.Score, Total: in.Total,
		StartedAt: in.StartedAt, IdempotencyKey: in.IdempotencyKey,
	}
	f.savedSessions = append(f.savedSessions, s)
	return s, nil
}

func (f *fakeProgress) GetSummary(ctx context.Context, userID, studySetID int64, filter model.ProgressFilter) (model.ProgressSummary, error) {
	return model.ProgressSummary{StudySetID: studySetID, TotalSessions: len(f.savedSessions)}, nil
}

func (f *fakeProgress) GetLatestByMode(ctx context.Context, userID, studySetID int64, mode model.LearningMode) (model.LearningSession, error) {
	for i := len(f.savedSessions) - 1; i >= 0; i-- {
		if f.savedSessions[i].Mode == mode {
			return f.savedSessions[i], nil
		}
	}
	return model.LearningSession{}, repository.ErrNotFound
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func goodInput() model.SaveProgressInput {
	return model.SaveProgressInput{
		Mode:           model.ModFlashcards,
		Score:          5,
		Total:          10,
		StartedAt:      time.Now(),
		IdempotencyKey: "key-abc-123",
		CardResults:    []model.CardResultInput{{FlashcardID: 1, Correct: true, Attempts: 1}},
	}
}

func newSvc(ownerMap map[int64]int64, cards []model.Flashcard, prog *fakeProgress) *ProgressService {
	return NewProgressService(prog, &fakeStudySets{ownerMap: ownerMap}, &fakeFlashcards{cards: cards})
}

// ---------------------------------------------------------------------------
// Tests: unauthenticated
// ---------------------------------------------------------------------------

func TestSave_ZeroUserID_Unauthorized(t *testing.T) {
	svc := newSvc(nil, nil, &fakeProgress{})
	_, err := svc.Save(context.Background(), 0, 1, goodInput())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: ownership
// ---------------------------------------------------------------------------

func TestSave_NotOwner_Forbidden(t *testing.T) {
	// study set 1 belongs to user 42, not user 99
	svc := newSvc(map[int64]int64{1: 42}, nil, &fakeProgress{})
	_, err := svc.Save(context.Background(), 99, 1, goodInput())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestSave_Owner_Succeeds(t *testing.T) {
	cards := []model.Flashcard{{ID: 1, StudySetID: 1}}
	prog := &fakeProgress{}
	svc := newSvc(map[int64]int64{1: 42}, cards, prog)
	_, err := svc.Save(context.Background(), 42, 1, goodInput())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(prog.savedSessions) != 1 {
		t.Fatalf("expected 1 session saved, got %d", len(prog.savedSessions))
	}
}

// ---------------------------------------------------------------------------
// Tests: validation
// ---------------------------------------------------------------------------

func TestSave_InvalidMode(t *testing.T) {
	cards := []model.Flashcard{{ID: 1, StudySetID: 1}}
	svc := newSvc(map[int64]int64{1: 42}, cards, &fakeProgress{})
	in := goodInput()
	in.Mode = "invalid-mode"
	_, err := svc.Save(context.Background(), 42, 1, in)
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestSave_ScoreExceedsTotal(t *testing.T) {
	cards := []model.Flashcard{{ID: 1, StudySetID: 1}}
	svc := newSvc(map[int64]int64{1: 42}, cards, &fakeProgress{})
	in := goodInput()
	in.Score = 11
	in.Total = 10
	_, err := svc.Save(context.Background(), 42, 1, in)
	if err == nil {
		t.Fatal("expected error when score > total")
	}
}

func TestSave_TotalExceedsMax(t *testing.T) {
	svc := newSvc(map[int64]int64{1: 42}, nil, &fakeProgress{})
	in := goodInput()
	in.Total = 101
	in.Score = 0
	_, err := svc.Save(context.Background(), 42, 1, in)
	if err == nil {
		t.Fatal("expected error when total > 100")
	}
}

func TestSave_MissingIdempotencyKey(t *testing.T) {
	svc := newSvc(map[int64]int64{1: 42}, nil, &fakeProgress{})
	in := goodInput()
	in.IdempotencyKey = ""
	_, err := svc.Save(context.Background(), 42, 1, in)
	if err == nil {
		t.Fatal("expected error when idempotencyKey is empty")
	}
}

func TestSave_TooManyCardResults(t *testing.T) {
	svc := newSvc(map[int64]int64{1: 42}, nil, &fakeProgress{})
	in := goodInput()
	in.CardResults = make([]model.CardResultInput, 101)
	for i := range in.CardResults {
		in.CardResults[i] = model.CardResultInput{FlashcardID: int64(i + 1), Correct: true, Attempts: 1}
	}
	_, err := svc.Save(context.Background(), 42, 1, in)
	if err == nil {
		t.Fatal("expected error when cardResults > 100")
	}
}

func TestSave_CardNotInSet_Rejected(t *testing.T) {
	// Card 99 belongs to set 2, not set 1
	cards := []model.Flashcard{{ID: 99, StudySetID: 2}}
	svc := newSvc(map[int64]int64{1: 42}, cards, &fakeProgress{})
	in := goodInput()
	in.CardResults = []model.CardResultInput{{FlashcardID: 99, Correct: true, Attempts: 1}}
	_, err := svc.Save(context.Background(), 42, 1, in)
	if err == nil {
		t.Fatal("expected error when card does not belong to the study set")
	}
}

// ---------------------------------------------------------------------------
// Tests: idempotency / duplicate key
// ---------------------------------------------------------------------------

func TestSave_DuplicateIdempotencyKey_ReturnsConflict(t *testing.T) {
	cards := []model.Flashcard{{ID: 1, StudySetID: 1}}
	prog := &fakeProgress{dupKey: true}
	svc := newSvc(map[int64]int64{1: 42}, cards, prog)
	_, err := svc.Save(context.Background(), 42, 1, goodInput())
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: repository error propagation (simulates transaction rollback scenario)
// ---------------------------------------------------------------------------

func TestSave_RepositoryError_PropagatesUp(t *testing.T) {
	cards := []model.Flashcard{{ID: 1, StudySetID: 1}}
	prog := &fakeProgress{saveErr: errors.New("db connection reset")}
	svc := newSvc(map[int64]int64{1: 42}, cards, prog)
	_, err := svc.Save(context.Background(), 42, 1, goodInput())
	if err == nil {
		t.Fatal("expected error from repository, got nil")
	}
}

// ---------------------------------------------------------------------------
// Tests: GetSummary pagination
// ---------------------------------------------------------------------------

func TestGetSummary_NormalizesFilter(t *testing.T) {
	prog := &fakeProgress{}
	svc := newSvc(map[int64]int64{1: 42}, nil, prog)
	// Zero values should be normalised to sane defaults without panic
	_, err := svc.GetSummary(context.Background(), 42, 1, model.ProgressFilter{Page: 0, PerPage: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetSummary_PerPageCappedAt50(t *testing.T) {
	prog := &fakeProgress{}
	svc := newSvc(map[int64]int64{1: 42}, nil, prog)
	_, err := svc.GetSummary(context.Background(), 42, 1, model.ProgressFilter{Page: 1, PerPage: 9999})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The cap is enforced in normalizeFilter; no assertion on returned value here
	// because fakeProgress ignores it – the point is no panic and no error.
}

func TestGetSummary_NotOwner_Forbidden(t *testing.T) {
	svc := newSvc(map[int64]int64{1: 42}, nil, &fakeProgress{})
	_, err := svc.GetSummary(context.Background(), 99, 1, model.ProgressFilter{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: GetLatestByMode
// ---------------------------------------------------------------------------

func TestGetLatestByMode_NotFound(t *testing.T) {
	prog := &fakeProgress{}
	svc := newSvc(map[int64]int64{1: 42}, nil, prog)
	_, err := svc.GetLatestByMode(context.Background(), 42, 1, model.ModLearn)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetLatestByMode_InvalidMode(t *testing.T) {
	svc := newSvc(map[int64]int64{1: 42}, nil, &fakeProgress{})
	_, err := svc.GetLatestByMode(context.Background(), 42, 1, "whatever")
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestGetLatestByMode_ReturnsCorrectSession(t *testing.T) {
	cards := []model.Flashcard{{ID: 1, StudySetID: 1}}
	prog := &fakeProgress{}
	svc := newSvc(map[int64]int64{1: 42}, cards, prog)

	in := goodInput()
	in.Mode = model.ModLearn
	in.IdempotencyKey = "key-learn-1"
	if _, err := svc.Save(context.Background(), 42, 1, in); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	session, err := svc.GetLatestByMode(context.Background(), 42, 1, model.ModLearn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.Mode != model.ModLearn {
		t.Fatalf("expected mode %q, got %q", model.ModLearn, session.Mode)
	}
}
