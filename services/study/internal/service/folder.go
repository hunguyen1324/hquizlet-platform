package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// FolderService orchestrates folder operations.
type FolderService struct {
	folders repository.Folders
	sets    repository.StudySets
}

// NewFolderService wires the service with its repositories.
func NewFolderService(folders repository.Folders, sets repository.StudySets) *FolderService {
	return &FolderService{folders: folders, sets: sets}
}

// List returns all folders for a user.
func (s *FolderService) List(ctx context.Context, userID int64) ([]model.Folder, error) {
	if userID <= 0 {
		return nil, ErrUnauthorized
	}
	return s.folders.List(ctx, userID)
}

// GetWithStudySets returns a folder with its study sets, verifying ownership.
func (s *FolderService) GetWithStudySets(ctx context.Context, id, userID int64) (model.Folder, error) {
	if err := requireUserID(userID); err != nil {
		return model.Folder{}, err
	}
	folder, err := s.folders.Get(ctx, id)
	if err != nil {
		return model.Folder{}, err
	}
	if folder.UserID != userID {
		return model.Folder{}, ErrForbidden
	}
	sets, err := s.folders.ListStudySets(ctx, id)
	if err != nil {
		return model.Folder{}, err
	}
	folder.StudySets = sets
	folder.StudySetCount = len(sets)
	return folder, nil
}

// Create validates and creates a new folder.
func (s *FolderService) Create(ctx context.Context, userID int64, in model.CreateFolderInput) (model.Folder, error) {
	if err := requireUserID(userID); err != nil {
		return model.Folder{}, err
	}
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" {
		return model.Folder{}, fmt.Errorf("%w: title is required", ErrValidation)
	}
	return s.folders.Create(ctx, userID, in)
}

// Update validates and updates a folder, checking ownership.
func (s *FolderService) Update(ctx context.Context, id, userID int64, in model.UpdateFolderInput) (model.Folder, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = strings.TrimSpace(in.Description)
	if in.Title == "" {
		return model.Folder{}, fmt.Errorf("%w: title is required", ErrValidation)
	}
	if err := s.checkOwner(ctx, id, userID); err != nil {
		return model.Folder{}, err
	}
	return s.folders.Update(ctx, id, in)
}

// Delete removes a folder, checking ownership.
func (s *FolderService) Delete(ctx context.Context, id, userID int64) error {
	if err := s.checkOwner(ctx, id, userID); err != nil {
		return err
	}
	return s.folders.Delete(ctx, id)
}

// AddStudySet links a study set to a folder, verifying both belong to userID.
func (s *FolderService) AddStudySet(ctx context.Context, folderID, studySetID, userID int64) error {
	if err := s.checkOwner(ctx, folderID, userID); err != nil {
		return err
	}
	ok, err := s.sets.IsOwner(ctx, studySetID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return s.folders.AddStudySet(ctx, folderID, studySetID)
}

// RemoveStudySet unlinks a study set from a folder, verifying ownership.
func (s *FolderService) RemoveStudySet(ctx context.Context, folderID, studySetID, userID int64) error {
	if err := s.checkOwner(ctx, folderID, userID); err != nil {
		return err
	}
	return s.folders.RemoveStudySet(ctx, folderID, studySetID)
}

func (s *FolderService) checkOwner(ctx context.Context, id, userID int64) error {
	if err := requireUserID(userID); err != nil {
		return err
	}
	ok, err := s.folders.IsOwner(ctx, id, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}

func requireUserID(userID int64) error {
	if userID <= 0 {
		return ErrUnauthorized
	}
	return nil
}
