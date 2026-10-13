package service

import (
	"context"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
)

// ClassAdminService backs the platform-wide admin console views of the class service (ADM-06).
// Read-only on purpose: takedown/disable of classes is a separate, audited action that is not
// implemented yet, so the console cannot pretend to have it.
type ClassAdminService struct {
	admin repository.AdminStore
}

func NewClassAdminService(admin repository.AdminStore) *ClassAdminService {
	return &ClassAdminService{admin: admin}
}

// ListClasses returns every class in the platform. Requires dashboard:ViewAdmin.
func (s *ClassAdminService) ListClasses(ctx context.Context, p authz.Principal, search string, limit, offset int) ([]*model.AdminClassRow, int, error) {
	if p.UserID <= 0 {
		return nil, 0, ErrUnauthorized
	}
	if !authz.Can(p, authz.ActionDashboardViewAdmin, authz.Resource{Type: "platform"}) {
		return nil, 0, ErrForbidden
	}
	rows, total, err := s.admin.ListAllClasses(ctx, search, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	if rows == nil {
		rows = []*model.AdminClassRow{}
	}
	return rows, total, nil
}
