package service

import (
	"context"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/subtitles"
	"strings"

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
	if err := s.checkOwner(ctx, studySetID, userID); err != nil {
		return nil, err
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
	for i := range in.Questions {
		if err := normalizeQuizQuestion(&in.Questions[i]); err != nil {
			return err
		}
	}
	return s.quizQuestions.BulkSave(ctx, studySetID, in.Questions)
}

func (s *QuizQuestionService) Update(ctx context.Context, setID, questionID, uid int64, q model.CreateQuizQuestionInput) error {
	if err := s.checkOwner(ctx, setID, uid); err != nil {
		return err
	}
	if strings.TrimSpace(q.QuestionText) == "" {
		return ErrValidation
	}
	if err := normalizeQuizQuestion(&q); err != nil {
		return err
	}
	return s.quizQuestions.Update(ctx, setID, questionID, q)
}

func normalizeQuizQuestion(q *model.CreateQuizQuestionInput) error {
	text, err := subtitles.Normalize(q.SRTText)
	if err != nil {
		return ErrValidation
	}
	q.SRTText = text
	switch q.QuestionType {
	case "multiple_choice", "true_false", "written", "paragraph", "sorting":
		return nil
	default:
		return ErrValidation
	}
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
	if !ok && !hasStudyWritePermission(ctx, userID, id) {
		return ErrForbidden
	}
	return nil
}

func (s *QuizQuestionService) RequireOwner(ctx context.Context, setID, uid int64) error {
	return s.checkOwner(ctx, setID, uid)
}
