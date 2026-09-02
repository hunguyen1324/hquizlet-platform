// Package repository defines interfaces and errors for class data access.
package repository

import (
	"context"
	"errors"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

// Sentinel errors returned by repositories.
var (
	ErrNotFound   = errors.New("resource not found")
	ErrConflict   = errors.New("resource already exists")
	ErrForbidden  = errors.New("forbidden")
	ErrValidation = errors.New("validation error")
	ErrInternal   = errors.New("internal error")
)

// ClassStore is the interface for class data access.
type ClassStore interface {
	Create(ctx context.Context, ownerID int64, input model.CreateClassInput) (*model.Class, error)
	GetByID(ctx context.Context, classID int64) (*model.Class, error)
	GetByInviteCode(ctx context.Context, code string) (*model.Class, error)
	ListByUserID(ctx context.Context, userID int64) ([]*model.ClassSummary, error)
	Update(ctx context.Context, classID int64, input model.UpdateClassInput) (*model.Class, error)
	Delete(ctx context.Context, classID int64) error
	ResetInviteCode(ctx context.Context, classID int64, newCode string) error
}

// MemberStore is the interface for member data access.
type MemberStore interface {
	Add(ctx context.Context, classID, userID int64, role string) (*model.ClassMember, error)
	ListByClass(ctx context.Context, classID int64) ([]*model.ClassMember, error)
	GetRole(ctx context.Context, classID, userID int64) (string, error)
	UpdateRole(ctx context.Context, classID, userID int64, role string) error
	Remove(ctx context.Context, classID, userID int64) error
	CountByClass(ctx context.Context, classID int64) (int, error)
}

// ClassStudySetStore is the interface for class study set data access.
type ClassStudySetStore interface {
	Add(ctx context.Context, classID, studySetID, addedByUserID int64) error
	List(ctx context.Context, classID int64) ([]*model.ClassStudySet, error)
	Remove(ctx context.Context, classID, studySetID int64) error
	CountByClass(ctx context.Context, classID int64) (int, error)
}

// ActivityStore is the interface for activity event data access.
type ActivityStore interface {
	Create(ctx context.Context, event model.ActivityEvent) error
	ListByUser(ctx context.Context, userID int64, limit int, cursor *model.ActivityCursor) ([]*model.ActivityEvent, error)
}

// Compile-time interface checks
var (
	_ ClassStore         = (*ClassRepository)(nil)
	_ MemberStore        = (*MemberRepository)(nil)
	_ ClassStudySetStore = (*ClassStudySetRepository)(nil)
	_ ActivityStore      = (*ActivityRepository)(nil)
)
