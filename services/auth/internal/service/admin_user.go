package service

import (
	"context"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
)

func (s *AuthService) GetAdminUser(ctx context.Context, p authz.Principal, id int64) (AdminUserView, error) {
	if err := requirePlatform(p, authz.ActionUserList); err != nil {
		return AdminUserView{}, err
	}
	users, err := s.repo.ListUsersByIDs(ctx, []int64{id})
	if err != nil {
		return AdminUserView{}, err
	}
	if len(users) == 0 {
		return AdminUserView{}, repository.ErrNotFound
	}
	u := users[0]
	roles,err:=s.repo.ListPlatformRoleKeys(ctx,u.ID);if err!=nil{return AdminUserView{},err}
	return AdminUserView{PlatformRoles:roles,ID: u.ID, Name: u.Name, Email: u.Email, Role: u.Role, Disabled: u.Disabled, CreatedAt: u.CreatedAt}, nil
}
