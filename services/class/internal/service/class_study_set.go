package service

import (
	"context"
	"errors"

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
func (s *ClassStudySetService) AddStudySet(ctx context.Context, classID, userID, studySetID int64) error {
	if userID <= 0 {
		return ErrUnauthorized
	}

	if err := s.requireTeacherOrOwner(ctx, classID, userID); err != nil {
		return err
	}

	if s.studyClient != nil {
		info, err := s.studyClient.GetStudySet(ctx, studySetID)
		if err != nil {
			// Study service unavailable — allow with warning
		} else if info == nil {
			return errors.New("study set not found")
		}
	}

	return s.studySets.Add(ctx, classID, studySetID, userID)
}

// ListStudySets returns all study sets assigned to a class.
func (s *ClassStudySetService) ListStudySets(ctx context.Context, classID, userID int64) ([]*model.ClassStudySet, error) {
	if userID <= 0 {
		return nil, ErrUnauthorized
	}

	if err := s.requireMember(ctx, classID, userID); err != nil {
		return nil, err
	}

	return s.studySets.List(ctx, classID)
}

// RemoveStudySet removes a study set from a class (owner/teacher only).
func (s *ClassStudySetService) RemoveStudySet(ctx context.Context, classID, userID, studySetID int64) error {
	if userID <= 0 {
		return ErrUnauthorized
	}

	if err := s.requireTeacherOrOwner(ctx, classID, userID); err != nil {
		return err
	}

	return s.studySets.Remove(ctx, classID, studySetID)
}

// CountByClass returns the number of study sets assigned to a class.
func (s *ClassStudySetService) CountByClass(ctx context.Context, classID int64) (int, error) {
	return s.studySets.CountByClass(ctx, classID)
}

func (s *ClassStudySetService) requireMember(ctx context.Context, classID, userID int64) error {
	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return err
	}
	if class.OwnerUserID == userID {
		return nil
	}

	role, err := s.members.GetRole(ctx, classID, userID)
	if err != nil {
		return err
	}
	if role == "" {
		return ErrForbidden
	}
	return nil
}

func (s *ClassStudySetService) requireTeacherOrOwner(ctx context.Context, classID, userID int64) error {
	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return err
	}
	if class.OwnerUserID == userID {
		return nil
	}

	role, err := s.members.GetRole(ctx, classID, userID)
	if err != nil {
		return err
	}
	if role != "teacher" && role != "owner" {
		return ErrForbidden
	}
	return nil
}
