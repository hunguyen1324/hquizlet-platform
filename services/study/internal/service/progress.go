// Package service – progress.go implements business logic for learning progress.
package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// ProgressService owns all business logic for saving and reading progress.
type ProgressService struct {
	progress repository.LearningProgress
	sets     repository.StudySets
	cards    repository.Flashcards
}

// NewProgressService returns a new ProgressService.
func NewProgressService(
	progress repository.LearningProgress,
	sets repository.StudySets,
	cards repository.Flashcards,
) *ProgressService {
	return &ProgressService{progress: progress, sets: sets, cards: cards}
}

// Save validates input, verifies ownership, checks card membership, then delegates
// to the repository for the atomic insert.
func (s *ProgressService) Save(
	ctx context.Context,
	userID, studySetID int64,
	in model.SaveProgressInput,
) (model.LearningSession, error) {
	if userID == 0 {
		return model.LearningSession{}, ErrUnauthorized
	}

	// Validate business rules.
	if err := validateSaveProgressInput(in); err != nil {
		return model.LearningSession{}, err
	}

	// Ownership: the authenticated user must own this study set.
	owned, err := s.sets.IsOwner(ctx, studySetID, userID)
	if err != nil {
		return model.LearningSession{}, err
	}
	if !owned {
		return model.LearningSession{}, ErrForbidden
	}

	// Verify every card belongs to this study set (prevents cross-set injection).
	if err := s.verifyCardsInSet(ctx, studySetID, in.CardResults); err != nil {
		return model.LearningSession{}, err
	}

	session, err := s.progress.Save(ctx, userID, studySetID, in)
	if errors.Is(err, repository.ErrDuplicateIdempotencyKey) {
		return model.LearningSession{}, ErrConflict
	}
	return session, err
}

// GetSummary returns progress summary for userID + studySetID.
// studySetID ownership is verified before querying progress data.
func (s *ProgressService) GetSummary(
	ctx context.Context,
	userID, studySetID int64,
	f model.ProgressFilter,
) (model.ProgressSummary, error) {
	if userID == 0 {
		return model.ProgressSummary{}, ErrUnauthorized
	}
	owned, err := s.sets.IsOwner(ctx, studySetID, userID)
	if err != nil {
		return model.ProgressSummary{}, err
	}
	if !owned {
		return model.ProgressSummary{}, ErrForbidden
	}

	f = normalizeFilter(f)
	return s.progress.GetSummary(ctx, userID, studySetID, f)
}

// GetLatestByMode returns the most recent session for the given mode.
func (s *ProgressService) GetLatestByMode(
	ctx context.Context,
	userID, studySetID int64,
	mode model.LearningMode,
) (model.LearningSession, error) {
	if userID == 0 {
		return model.LearningSession{}, ErrUnauthorized
	}
	if !mode.Valid() {
		return model.LearningSession{}, fmt.Errorf("invalid mode %q", mode)
	}
	owned, err := s.sets.IsOwner(ctx, studySetID, userID)
	if err != nil {
		return model.LearningSession{}, err
	}
	if !owned {
		return model.LearningSession{}, ErrForbidden
	}
	return s.progress.GetLatestByMode(ctx, userID, studySetID, mode)
}

func (s *ProgressService) GetLatest(ctx context.Context, userID, studySetID int64) ([]model.LearningSession, error) {
	if userID == 0 { return nil, ErrUnauthorized }
	owned, err := s.sets.IsOwner(ctx, studySetID, userID)
	if err != nil { return nil, err }
	if !owned { return nil, ErrForbidden }
	return s.progress.GetLatest(ctx, userID, studySetID)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validateSaveProgressInput(in model.SaveProgressInput) error {
	var errs []string

	if !in.Mode.Valid() {
		errs = append(errs, fmt.Sprintf("mode %q is not valid (must be flashcards|learn|test|match)", in.Mode))
	}
	if in.Score < 0 {
		errs = append(errs, "score must be >= 0")
	}
	if in.Total < 0 || in.Total > 100 {
		errs = append(errs, "total must be between 0 and 100")
	}
	if in.Score > in.Total {
		errs = append(errs, "score must be <= total")
	}
	if in.IdempotencyKey == "" {
		errs = append(errs, "idempotencyKey is required")
	}
	if in.StartedAt.IsZero() {
		errs = append(errs, "startedAt is required")
	}
	if len(in.CardResults) > 100 {
		errs = append(errs, "cardResults must not exceed 100 items per request")
	}
	for i, cr := range in.CardResults {
		if cr.FlashcardID <= 0 {
			errs = append(errs, fmt.Sprintf("cardResults[%d]: flashcardId must be > 0", i))
		}
		if cr.Attempts < 1 || cr.Attempts > 100 {
			errs = append(errs, fmt.Sprintf("cardResults[%d]: attempts must be between 1 and 100", i))
		}
		if cr.ResponseTimeMs != nil && *cr.ResponseTimeMs < 0 {
			errs = append(errs, fmt.Sprintf("cardResults[%d]: responseTimeMs must be >= 0", i))
		}
	}

	if len(errs) > 0 {
		return ProgressValidationError{Message: strings.Join(errs, "; ")}
	}
	return nil
}

type ProgressValidationError struct { Message string }
func (e ProgressValidationError) Error() string { return e.Message }
func IsProgressValidationError(err error) bool {
	var target ProgressValidationError
	return errors.As(err, &target)
}

// verifyCardsInSet loads all flashcards for the study set and checks that every
// submitted cardID is present. A missing or foreign card causes the entire save to fail.
func (s *ProgressService) verifyCardsInSet(
	ctx context.Context,
	studySetID int64,
	results []model.CardResultInput,
) error {
	if len(results) == 0 {
		return nil
	}

	cards, err := s.cards.ListByStudySet(ctx, studySetID)
	if err != nil {
		return err
	}
	validIDs := make(map[int64]struct{}, len(cards))
	for _, c := range cards {
		validIDs[c.ID] = struct{}{}
	}

	for i, cr := range results {
		if _, ok := validIDs[cr.FlashcardID]; !ok {
			return fmt.Errorf("cardResults[%d]: flashcard %d does not belong to study set %d",
				i, cr.FlashcardID, studySetID)
		}
	}
	return nil
}

func normalizeFilter(f model.ProgressFilter) model.ProgressFilter {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PerPage < 1 {
		f.PerPage = 20
	}
	if f.PerPage > 50 {
		f.PerPage = 50
	}
	return f
}

// ErrConflict is returned on idempotency key collision (duplicate submit).
var ErrConflict = errors.New("conflict: duplicate idempotency key")
