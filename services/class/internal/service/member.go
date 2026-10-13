package service

import (
	"context"
	"errors"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
)

// MemberService handles member management operations.
type MemberService struct {
	classes repository.ClassStore
	members repository.MemberStore
}

// NewMemberService creates a new MemberService.
func NewMemberService(classes repository.ClassStore, members repository.MemberStore) *MemberService {
	return &MemberService{classes: classes, members: members}
}

// ListMembers returns all members of a class (any class member).
func (s *MemberService) ListMembers(ctx context.Context, p authz.Principal, classID int64) ([]*model.ClassMember, error) {
	if _, err := s.authorize(ctx, p, classID, authz.ActionClassMemberList); err != nil {
		return nil, err
	}
	return s.members.ListByClass(ctx, classID)
}

// AddMember adds a member manually (owner/teacher only).
func (s *MemberService) AddMember(ctx context.Context, p authz.Principal, classID, targetUserID int64, role string) (*model.ClassMember, error) {
	if _, err := s.authorize(ctx, p, classID, authz.ActionClassMemberAdd); err != nil {
		return nil, err
	}

	role = strings.TrimSpace(strings.ToLower(role))
	if !model.ValidRoles[role] || role == "owner" {
		return nil, errors.New("invalid role: must be 'teacher' or 'student'")
	}

	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return nil, err
	}
	memberCount, err := s.members.CountByClass(ctx, classID)
	if err != nil {
		return nil, err
	}
	if memberCount+1 > class.MaxMembers {
		return nil, errors.New("class is full")
	}

	return s.members.Add(ctx, classID, targetUserID, role)
}

// JoinByCode joins a class using an invite code.
func (s *MemberService) JoinByCode(ctx context.Context, code string, userID int64) (*model.JoinClassResponse, error) {
	if userID <= 0 {
		return nil, ErrUnauthorized
	}

	code = strings.TrimSpace(strings.ToUpper(code))
	if code == "" {
		return nil, errors.New("invite code is required")
	}

	class, err := s.classes.GetByInviteCode(ctx, code)
	if err != nil {
		return nil, err
	}

	if class.OwnerUserID == userID {
		return nil, errors.New("owner cannot join their own class")
	}

	memberCount, err := s.members.CountByClass(ctx, class.ID)
	if err != nil {
		return nil, err
	}
	if memberCount+1 > class.MaxMembers {
		return nil, errors.New("class is full")
	}

	existing, err := s.members.GetRole(ctx, class.ID, userID)
	if err != nil {
		return nil, err
	}
	if existing != "" {
		return nil, ErrConflict
	}

	member, err := s.members.Add(ctx, class.ID, userID, "student")
	if err != nil {
		return nil, err
	}

	return &model.JoinClassResponse{
		ClassID:   class.ID,
		ClassName: class.Name,
		MyRole:    member.Role,
		JoinedAt:  member.JoinedAt,
	}, nil
}

// UpdateRole changes a member's role (owner/teacher; never the owner's own record).
func (s *MemberService) UpdateRole(ctx context.Context, p authz.Principal, classID, targetUserID int64, newRole string) error {
	class, err := s.authorize(ctx, p, classID, authz.ActionClassMemberUpdateRole)
	if err != nil {
		return err
	}

	newRole = strings.TrimSpace(strings.ToLower(newRole))
	if !model.ValidRoles[newRole] || newRole == "owner" {
		return errors.New("invalid role: must be 'teacher' or 'student'")
	}

	if targetUserID == class.OwnerUserID {
		return ErrForbidden
	}
	if targetUserID == p.UserID {
		return errors.New("cannot change your own role")
	}

	return s.members.UpdateRole(ctx, classID, targetUserID, newRole)
}

// RemoveMember removes a member from a class (owner/teacher; never the owner).
func (s *MemberService) RemoveMember(ctx context.Context, p authz.Principal, classID, targetUserID int64) error {
	class, err := s.authorize(ctx, p, classID, authz.ActionClassMemberRemove)
	if err != nil {
		return err
	}

	if targetUserID == class.OwnerUserID {
		return ErrForbidden
	}
	if targetUserID == p.UserID {
		return errors.New("use leave to remove yourself")
	}

	return s.members.Remove(ctx, classID, targetUserID)
}

// LeaveClass allows a non-owner member to leave a class.
func (s *MemberService) LeaveClass(ctx context.Context, classID, userID int64) error {
	if userID <= 0 {
		return ErrUnauthorized
	}

	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return err
	}
	if class.OwnerUserID == userID {
		return errors.New("owner cannot leave class")
	}

	return s.members.Remove(ctx, classID, userID)
}

// CountByClass returns the number of members in a class.
func (s *MemberService) CountByClass(ctx context.Context, classID int64) (int, error) {
	return s.members.CountByClass(ctx, classID)
}

// GetRole returns the role of a user in a class.
func (s *MemberService) GetRole(ctx context.Context, classID, userID int64) (string, error) {
	return s.members.GetRole(ctx, classID, userID)
}

// authorize resolves the caller's class role and checks the requested action with authz.Can.
func (s *MemberService) authorize(ctx context.Context, p authz.Principal, classID int64, action authz.Action) (*model.Class, error) {
	if p.UserID <= 0 {
		return nil, ErrUnauthorized
	}
	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return nil, err
	}
	role, err := s.members.GetRole(ctx, classID, p.UserID)
	if err != nil || role == "" {
		if class.OwnerUserID == p.UserID {
			role = "owner"
		} else {
			return nil, ErrForbidden
		}
	}
	p.ClassID = classID
	p.ClassRole = role
	res := authz.Resource{Type: "class", ID: classID, ClassID: classID, OwnerUserID: class.OwnerUserID}
	if !authz.Can(p, action, res) {
		return nil, ErrForbidden
	}
	return class, nil
}
