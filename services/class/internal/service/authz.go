package service

import (
	"context"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

func (s *ClassService) Permissions(ctx context.Context, p authz.Principal, classID int64) ([]authz.Action, error) {
	res, class, err := s.classResource(ctx, classID)
	if err != nil {
		return nil, err
	}
	p.ClassID = classID
	p.ClassRole = s.memberRole(ctx, classID, p.UserID, class)
	if !authz.Can(p, authz.ActionClassRead, res) {
		return nil, ErrForbidden
	}
	// The progress action in this UI contract is always the caller's own progress.
	res.OwnerUserID = p.UserID
	return authz.EffectiveActions(p, res), nil
}

func (s *ClassService) classResource(ctx context.Context, classID int64) (authz.Resource, *model.Class, error) {
	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return authz.Resource{}, nil, err
	}
	return authz.Resource{
		Type:        "class",
		ID:          classID,
		ClassID:     classID,
		OwnerUserID: class.OwnerUserID,
	}, class, nil
}

func (s *ClassService) memberRole(ctx context.Context, classID, userID int64, class *model.Class) string {
	role, err := s.members.GetRole(ctx, classID, userID)
	if err != nil || role == "" {
		if class != nil && class.OwnerUserID == userID {
			return "owner"
		}
		return ""
	}
	return role
}

func (s *ClassService) authorizeClass(ctx context.Context, p authz.Principal, classID int64, action authz.Action) (*model.Class, error) {
	if p.UserID <= 0 {
		return nil, ErrUnauthorized
	}
	res, class, err := s.classResource(ctx, classID)
	if err != nil {
		return nil, err
	}
	p.ClassID = classID
	p.ClassRole = s.memberRole(ctx, classID, p.UserID, class)
	if !authz.Can(p, action, res) {
		return nil, ErrForbidden
	}
	return class, nil
}
