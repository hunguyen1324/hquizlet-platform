package service

import (
	"context"
	"errors"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// FlashcardService orchestrates flashcard operations.
type FlashcardService struct {
	sets  repository.StudySets
	cards repository.Flashcards
}

// NewFlashcardService wires the service with its repositories.
func NewFlashcardService(sets repository.StudySets, cards repository.Flashcards) *FlashcardService {
	return &FlashcardService{sets: sets, cards: cards}
}

// Create validates input and creates a flashcard inside a study set.
func (s *FlashcardService) Create(ctx context.Context, studySetID, userID int64, in model.CreateFlashcardInput) (model.Flashcard, error) {
	in.Term = strings.TrimSpace(in.Term)
	in.Definition = strings.TrimSpace(in.Definition)
	if in.Term == "" || in.Definition == "" {
		return model.Flashcard{}, errors.New("term and definition are required")
	}
	if err := s.checkSetOwner(ctx, studySetID, userID); err != nil {
		return model.Flashcard{}, err
	}
	return s.cards.Create(ctx, studySetID, in)
}

// Update validates and updates a flashcard, checking ownership via its study set.
func (s *FlashcardService) Update(ctx context.Context, cardID, userID int64, in model.UpdateFlashcardInput) (model.Flashcard, error) {
	in.Term = strings.TrimSpace(in.Term)
	in.Definition = strings.TrimSpace(in.Definition)
	if in.Term == "" || in.Definition == "" {
		return model.Flashcard{}, errors.New("term and definition are required")
	}
	card, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return model.Flashcard{}, err
	}
	if err := s.checkSetOwner(ctx, card.StudySetID, userID); err != nil {
		return model.Flashcard{}, err
	}
	return s.cards.Update(ctx, cardID, in)
}

// ToggleStar flips the starred flag, checking ownership.
func (s *FlashcardService) ToggleStar(ctx context.Context, cardID, userID int64) (model.Flashcard, error) {
	card, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return model.Flashcard{}, err
	}
	if err := s.checkSetOwner(ctx, card.StudySetID, userID); err != nil {
		return model.Flashcard{}, err
	}
	return s.cards.ToggleStar(ctx, cardID)
}

// Delete removes a flashcard, checking ownership.
func (s *FlashcardService) Delete(ctx context.Context, cardID, userID int64) error {
	card, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return err
	}
	if err := s.checkSetOwner(ctx, card.StudySetID, userID); err != nil {
		return err
	}
	return s.cards.Delete(ctx, cardID)
}

// BulkSave creates/updates/deletes multiple flashcards in a single transaction.
func (s *FlashcardService) BulkSave(ctx context.Context, studySetID, userID int64, in model.BulkSaveFlashcardsInput) (model.BulkSaveResult, error) {
	if err := s.checkSetOwner(ctx, studySetID, userID); err != nil {
		return model.BulkSaveResult{}, err
	}
	for _, item := range in.Cards {
		if item.Delete {
			continue
		}
		if strings.TrimSpace(item.Term) == "" || strings.TrimSpace(item.Definition) == "" {
			return model.BulkSaveResult{}, errors.New("each card must have term and definition")
		}
	}
	return s.cards.BulkSave(ctx, studySetID, in.Cards)
}

func (s *FlashcardService) checkSetOwner(ctx context.Context, setID, userID int64) error {
	if userID == 0 {
		return nil
	}
	ok, err := s.sets.IsOwner(ctx, setID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
