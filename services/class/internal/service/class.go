// Package service contains business logic for the class service.
package service

import (
	"context"
	"errors"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
)

// Sentinel errors for the service layer.
var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrValidation   = errors.New("validation error")
	ErrConflict     = errors.New("conflict")
	ErrNotFound     = errors.New("not found")
)

// ClassService handles class CRUD operations.
type ClassService struct {
	classes repository.ClassStore
	members repository.MemberStore
}

// NewClassService creates a new ClassService.
func NewClassService(classes repository.ClassStore, members repository.MemberStore) *ClassService {
	return &ClassService{classes: classes, members: members}
}

// Create creates a new class with the owner as the initial member.
func (s *ClassService) Create(ctx context.Context, ownerID int64, input model.CreateClassInput) (*model.Class, error) {
	if ownerID <= 0 {
		return nil, ErrUnauthorized
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)

	if input.Name == "" {
		return nil, errors.New("name is required")
	}
	if len(input.Name) > 120 {
		return nil, errors.New("name must be at most 120 characters")
	}
	if input.MaxMembers <= 0 {
		input.MaxMembers = 100
	}
	if input.MaxMembers < 2 || input.MaxMembers > 1000 {
		return nil, errors.New("max members must be between 2 and 1000")
	}

	class, err := s.classes.Create(ctx, ownerID, input)
	if err != nil {
		return nil, err
	}

	// Add owner as a member
	if _, err := s.members.Add(ctx, class.ID, ownerID, "owner"); err != nil {
		return class, nil
	}

	return class, nil
}

// GetByID returns a class if the user is a member.
func (s *ClassService) GetByID(ctx context.Context, classID, userID int64) (*model.Class, error) {
	if userID <= 0 {
		return nil, ErrUnauthorized
	}

	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return nil, err
	}

	role, err := s.members.GetRole(ctx, classID, userID)
	if err != nil {
		return class, nil
	}
	if role == "" && class.OwnerUserID != userID {
		return nil, ErrForbidden
	}

	return class, nil
}

// ListByUserID returns all classes the user is a member of.
func (s *ClassService) ListByUserID(ctx context.Context, userID int64) ([]*model.ClassSummary, error) {
	if userID <= 0 {
		return nil, ErrUnauthorized
	}
	return s.classes.ListByUserID(ctx, userID)
}

// Update modifies a class (owner only).
func (s *ClassService) Update(ctx context.Context, classID, userID int64, input model.UpdateClassInput) (*model.Class, error) {
	if userID <= 0 {
		return nil, ErrUnauthorized
	}

	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return nil, err
	}

	if class.OwnerUserID != userID {
		return nil, ErrForbidden
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)

	if input.Name == "" {
		return nil, errors.New("name is required")
	}
	if len(input.Name) > 120 {
		return nil, errors.New("name must be at most 120 characters")
	}

	return s.classes.Update(ctx, classID, input)
}

// Delete removes a class and all its members/study sets (owner only).
func (s *ClassService) Delete(ctx context.Context, classID, userID int64) error {
	if userID <= 0 {
		return ErrUnauthorized
	}

	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return err
	}

	if class.OwnerUserID != userID {
		return ErrForbidden
	}

	return s.classes.Delete(ctx, classID)
}

// ResetInviteCode generates a new invite code (owner only).
func (s *ClassService) ResetInviteCode(ctx context.Context, classID, userID int64) (string, error) {
	if userID <= 0 {
		return "", ErrUnauthorized
	}

	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return "", err
	}

	if class.OwnerUserID != userID {
		return "", ErrForbidden
	}

	cr, ok := s.classes.(*repository.ClassRepository)
	if !ok {
		return "", ErrForbidden
	}

	newCode, err := cr.GenerateUniqueInviteCode(ctx)
	if err != nil {
		return "", err
	}

	if err := s.classes.ResetInviteCode(ctx, classID, newCode); err != nil {
		return "", err
	}

	return newCode, nil
}
