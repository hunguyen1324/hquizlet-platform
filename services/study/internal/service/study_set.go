// Package service contains business logic for the study service.
// Handlers should never call repository directly; all logic goes here.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

type StudySetService struct {
	sets  repository.StudySets
	cards repository.Flashcards
}

func NewStudySetService(sets repository.StudySets, cards repository.Flashcards) *StudySetService {
	return &StudySetService{sets: sets, cards: cards}
}

// List returns only study sets owned by the authenticated user.
func (s *StudySetService) List(ctx context.Context, userID int64) ([]model.StudySet, error) {
	if err := requireUserID(userID); err != nil {
		return nil, err
	}
	return s.sets.List(ctx, userID)
}

// ListWithFilter returns only study sets owned by the authenticated user.
func (s *StudySetService) ListWithFilter(ctx context.Context, userID int64, f model.StudySetFilter) (model.StudySetListResult, error) {
	if err := requireUserID(userID); err != nil {
		return model.StudySetListResult{}, err
	}
	return s.sets.ListWithFilter(ctx, userID, f)
}

// GetWithCards returns a study set along with its flashcards.
// The study set must belong to userID; ownership is enforced by the repository query
// (GetOwned scopes the SQL lookup itself, so there is no separate app-level bypass window).
func (s *StudySetService) GetWithCards(ctx context.Context, id, userID int64) (model.StudySet, error) {
	if err := requireUserID(userID); err != nil {
		return model.StudySet{}, err
	}
	set, err := s.sets.GetOwned(ctx, id, userID)
	if err != nil {
		return model.StudySet{}, err
	}
	cards, err := s.cards.ListByStudySet(ctx, id)
	if err != nil {
		return model.StudySet{}, err
	}
	set.Flashcards = cards
	return set, nil
}

func (s *StudySetService) Create(ctx context.Context, userID int64, in model.CreateStudySetInput) (model.StudySet, error) {
	if err := requireUserID(userID); err != nil {
		return model.StudySet{}, err
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" {
		return model.StudySet{}, errors.New("title is required")
	}
	return s.sets.Create(ctx, userID, in)
}

func (s *StudySetService) Update(ctx context.Context, id, userID int64, in model.UpdateStudySetInput) (model.StudySet, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" {
		return model.StudySet{}, errors.New("title is required")
	}
	if err := s.checkOwner(ctx, id, userID); err != nil {
		return model.StudySet{}, err
	}
	return s.sets.Update(ctx, id, in)
}

func (s *StudySetService) Delete(ctx context.Context, id, userID int64) error {
	if err := s.checkOwner(ctx, id, userID); err != nil {
		return err
	}
	return s.sets.Delete(ctx, id)
}

// checkOwner never accepts zero/invalid user IDs as an authorization bypass.
func (s *StudySetService) checkOwner(ctx context.Context, id, userID int64) error {
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

// ErrUnauthorized is returned when no authenticated user ID is available.
var ErrUnauthorized = errors.New("unauthorized")
var ErrForbidden = errors.New("forbidden")
