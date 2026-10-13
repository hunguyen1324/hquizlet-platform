package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
)

type PermissionsResponse struct {
	PlatformRoles []string `json:"platformRoles"`
	LegacyRole    string   `json:"legacyRole"`
	Actions       []string `json:"actions"`
	ClassID       *int64   `json:"classId,omitempty"`
}

func (s *AuthService) Permissions(ctx context.Context, token string, classID int64) (PermissionsResponse, error) {
	identity, u, err := s.verifiedIdentity(ctx, token)
	if err != nil {
		return PermissionsResponse{}, err
	}
	roles := identity.PlatformRoles
	if len(roles) == 0 {
		roles, _ = s.repo.ListPlatformRoleKeys(ctx, u.ID)
	}
	p := authz.Principal{
		UserID:        u.ID,
		LegacyRole:    identity.Role,
		PlatformRoles: roles,
	}
	res := authz.Resource{Type: "platform"}
	if classID > 0 {
		res = authz.Resource{Type: "class", ID: classID, ClassID: classID}
		p.ClassID = classID
	}
	actions := authz.EffectiveActions(p, res)
	actionStrs := make([]string, len(actions))
	for i, a := range actions {
		actionStrs[i] = string(a)
	}
	out := PermissionsResponse{
		PlatformRoles: roles,
		LegacyRole:    identity.Role,
		Actions:       actionStrs,
	}
	if classID > 0 {
		out.ClassID = &classID
	}
	return out, nil
}

func platformRoleForRegistration(accountType string) string {
	switch strings.ToLower(strings.TrimSpace(accountType)) {
	case "teacher", "giaovien", "giáo viên":
		return authz.RoleTeacher
	default:
		return authz.RoleStudent
	}
}

func legacyRoleFromPlatformRoles(roles []string, usersRole string) string {
	for _, r := range roles {
		if r == authz.RolePlatformAdmin {
			return "admin"
		}
	}

	return "user"
}

func (s *AuthService) loadPlatformRoles(ctx context.Context, u model.User) ([]string, error) {
	roles, err := s.repo.ListPlatformRoleKeys(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		if store, ok := s.repo.(interface {
			HasPlatformAssignment(context.Context, int64) (bool, error)
		}); ok {
			exists, err := store.HasPlatformAssignment(ctx, u.ID)
			if err != nil {
				return nil, err
			}
			if exists {
				return []string{}, nil
			}
		}
	}
	if len(roles) == 0 && u.Role == "admin" {
		return []string{authz.RolePlatformAdmin}, nil
	}
	if len(roles) == 0 {
		return []string{authz.RoleStudent}, nil
	}
	return roles, nil
}

func auditResourceUser(id int64) string {
	return "hq:user/" + strconv.FormatInt(id, 10)
}
