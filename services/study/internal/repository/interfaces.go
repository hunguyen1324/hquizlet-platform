// Package repository defines interfaces for all data access.
// Services depend on these interfaces, not on concrete types,
// enabling easy unit testing with fakes.
package repository

import (
	"context"
	"errors"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// StudySets is the interface for study set data access.
type StudySets interface {
	List(ctx context.Context, userID int64) ([]model.StudySet, error)
	ListAll(ctx context.Context) ([]model.StudySet, error)
	ListWithFilter(ctx context.Context, userID int64, f model.StudySetFilter) (model.StudySetListResult, error)
	Get(ctx context.Context, id int64) (model.StudySet, error)
	GetOwned(ctx context.Context, id, userID int64) (model.StudySet, error)
	Create(ctx context.Context, userID int64, in model.CreateStudySetInput) (model.StudySet, error)
	Update(ctx context.Context, id int64, in model.UpdateStudySetInput) (model.StudySet, error)
	Delete(ctx context.Context, id int64) error
	IsOwner(ctx context.Context, id, userID int64) (bool, error)
}

// Flashcards is the interface for flashcard data access.
type Flashcards interface {
	ListByStudySet(ctx context.Context, studySetID int64) ([]model.Flashcard, error)
	Get(ctx context.Context, id int64) (model.Flashcard, error)
	Create(ctx context.Context, studySetID int64, in model.CreateFlashcardInput) (model.Flashcard, error)
	Update(ctx context.Context, id int64, in model.UpdateFlashcardInput) (model.Flashcard, error)
	ToggleStar(ctx context.Context, id int64) (model.Flashcard, error)
	Delete(ctx context.Context, id int64) error
	BulkSave(ctx context.Context, studySetID int64, items []model.BulkFlashcardItem) (model.BulkSaveResult, error)
}

// Folders is the interface for folder data access.
type Folders interface {
	List(ctx context.Context, userID int64) ([]model.Folder, error)
	Get(ctx context.Context, id int64) (model.Folder, error)
	Create(ctx context.Context, userID int64, in model.CreateFolderInput) (model.Folder, error)
	Update(ctx context.Context, id int64, in model.UpdateFolderInput) (model.Folder, error)
	Delete(ctx context.Context, id int64) error
	IsOwner(ctx context.Context, id, userID int64) (bool, error)
	ListStudySets(ctx context.Context, folderID int64) ([]model.StudySet, error)
	AddStudySet(ctx context.Context, folderID, studySetID int64) error
	RemoveStudySet(ctx context.Context, folderID, studySetID int64) error
}

// LearningProgress is the interface for progress data access.
// Save must be atomic: it inserts the session and all card results in one transaction,
// rolling back entirely on any error.
type LearningProgress interface {
	// Save inserts a new session + card results atomically.
	// Returns ErrDuplicateIdempotencyKey if (userID, idempotencyKey) already exists.
	Save(ctx context.Context, userID, studySetID int64, in model.SaveProgressInput) (model.LearningSession, error)
	// GetSummary returns aggregate stats + paginated history for one user+set.
	GetSummary(ctx context.Context, userID, studySetID int64, f model.ProgressFilter) (model.ProgressSummary, error)
	// GetLatestByMode returns the most-recent session for the given mode, or ErrNotFound.
	GetLatestByMode(ctx context.Context, userID, studySetID int64, mode model.LearningMode) (model.LearningSession, error)
	GetLatest(ctx context.Context, userID, studySetID int64) ([]model.LearningSession, error)
}

// QuizQuestions is the interface for quiz question data access.
type QuizQuestions interface {
	ListByStudySet(ctx context.Context, studySetID int64) ([]model.QuizQuestion, error)
	BulkSave(ctx context.Context, studySetID int64, questions []model.CreateQuizQuestionInput) error
	DeleteByStudySet(ctx context.Context, studySetID int64) error
}

// ErrDuplicateIdempotencyKey is returned when a retry uses the same idempotency key.
var ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")

// Compile-time interface checks.
var (
	_ StudySets       = (*StudySetRepository)(nil)
	_ Flashcards      = (*FlashcardRepository)(nil)
	_ Folders         = (*FolderRepository)(nil)
	_ LearningProgress = (*LearningProgressRepository)(nil)
	_ QuizQuestions   = (*QuizQuestionRepository)(nil)
)
