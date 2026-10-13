package service

import (
	"context"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

// AdminService backs the platform-wide content listing used by the admin console (ADM-06).
// Read-only: moderation (hide/remove) is not implemented, so it is not offered here.
type AdminService struct {
	admin repository.AdminStore
}

func NewAdminService(admin repository.AdminStore) *AdminService {
	return &AdminService{admin: admin}
}

// ListStudySets returns every study set in the platform. Requires dashboard:ViewAdmin.
func (s *AdminService) ListStudySets(ctx context.Context, p authz.Principal, search string, limit, offset int) ([]*model.AdminStudySetRow, int, error) {
	if p.UserID <= 0 {
		return nil, 0, ErrUnauthorized
	}
	if !authz.Can(p, authz.ActionDashboardViewAdmin, authz.Resource{Type: "platform"}) {
		return nil, 0, ErrForbidden
	}
	rows, total, err := s.admin.ListAllStudySets(ctx, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if rows == nil {
		rows = []*model.AdminStudySetRow{}
	}
	return rows, total, nil
}
