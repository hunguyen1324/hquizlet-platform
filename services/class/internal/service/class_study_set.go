package service

import (
	"context"
	"fmt"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/client"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
)

// ClassStudySetService handles study set assignment operations.
type ClassStudySetService struct {
	classes     repository.ClassStore
	members     repository.MemberStore
	studySets   repository.ClassStudySetStore
	studyClient *client.StudyClient
}

// NewClassStudySetService creates a new ClassStudySetService.
func NewClassStudySetService(
	classes repository.ClassStore,
	members repository.MemberStore,
	studySets repository.ClassStudySetStore,
	studyClient *client.StudyClient,
) *ClassStudySetService {
	return &ClassStudySetService{
		classes:     classes,
		members:     members,
		studySets:   studySets,
		studyClient: studyClient,
	}
}

// AddStudySet assigns a study set to a class (owner/teacher only).
func (s *ClassStudySetService) AddStudySet(ctx context.Context, p authz.Principal, classID, studySetID int64) error {
	if p.UserID <= 0 {
		return ErrUnauthorized
	}

	if err := s.authorize(ctx, p, classID, authz.ActionClassContentAssign); err != nil {
		return err
	}

	if s.studyClient != nil {
		info, err := s.studyClient.GetStudySet(ctx, studySetID)
		if err != nil {
			return fmt.Errorf("verify assigned study set: %w", err)
		} else if info == nil {
			return ErrNotFound
		}
	}

	return s.studySets.Add(ctx, classID, studySetID, p.UserID)
}

// ListStudySets returns all study sets assigned to a class.
func (s *ClassStudySetService) ListStudySets(ctx context.Context, p authz.Principal, classID int64) ([]*model.ClassStudySet, error) {
	if p.UserID <= 0 {
		return nil, ErrUnauthorized
	}

	if err := s.authorize(ctx, p, classID, authz.ActionClassContentView); err != nil {
		return nil, err
	}

	return s.studySets.List(ctx, classID)
}

// RemoveStudySet removes a study set from a class (owner/teacher only).
func (s *ClassStudySetService) RemoveStudySet(ctx context.Context, p authz.Principal, classID, studySetID int64) error {
	if p.UserID <= 0 {
		return ErrUnauthorized
	}

	if err := s.authorize(ctx, p, classID, authz.ActionClassContentUnassign); err != nil {
		return err
	}

	return s.studySets.Remove(ctx, classID, studySetID)
}

// CountByClass returns the number of study sets assigned to a class.
func (s *ClassStudySetService) CountByClass(ctx context.Context, classID int64) (int, error) {
	return s.studySets.CountByClass(ctx, classID)
}

func (s *ClassStudySetService) authorize(ctx context.Context, p authz.Principal, classID int64, action authz.Action) error {
	authorizer := &ClassService{classes: s.classes, members: s.members}
	_, err := authorizer.authorizeClass(ctx, p, classID, action)
	return err
}
