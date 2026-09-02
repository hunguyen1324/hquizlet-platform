package service

import (
	"context"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// QuizQuestionService handles quiz question business logic.
type QuizQuestionService struct {
	quizQuestions repository.QuizQuestions
	sets          repository.StudySets
}

// NewQuizQuestionService creates a new service.
func NewQuizQuestionService(quizQuestions repository.QuizQuestions, sets repository.StudySets) *QuizQuestionService {
	return &QuizQuestionService{quizQuestions: quizQuestions, sets: sets}
}

// ListByStudySet returns all quiz questions for a study set.
// Visibility enforcement: private sets can only be accessed by the owner.
func (s *QuizQuestionService) ListByStudySet(ctx context.Context, studySetID, userID int64) ([]model.QuizQuestion, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	set, err := s.sets.Get(ctx, studySetID)
	if err != nil {
		return nil, err
	}
	if set.Visibility == "private" && set.UserID != userID {
		return nil, ErrForbidden
	}
	return s.quizQuestions.ListByStudySet(ctx, studySetID)
}

// BulkSave replaces all quiz questions for a study set. Only the owner can do this.
func (s *QuizQuestionService) BulkSave(ctx context.Context, studySetID, userID int64, in model.BulkSaveQuizQuestionsInput) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	if err := s.checkOwner(ctx, studySetID, userID); err != nil {
		return err
	}
	// Validate each question
	for _, q := range in.Questions {
		switch q.QuestionType {
		case "multiple_choice", "true_false", "written", "paragraph", "sorting":
			// valid
		default:
			return ErrValidation
		}
	}
	return s.quizQuestions.BulkSave(ctx, studySetID, in.Questions)
}

// DeleteByStudySet removes all quiz questions for a study set. Only the owner can do this.
func (s *QuizQuestionService) DeleteByStudySet(ctx context.Context, studySetID, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	if err := s.checkOwner(ctx, studySetID, userID); err != nil {
		return err
	}
	return s.quizQuestions.DeleteByStudySet(ctx, studySetID)
}

func (s *QuizQuestionService) checkOwner(ctx context.Context, id, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	ok, err := s.sets.IsOwner(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
