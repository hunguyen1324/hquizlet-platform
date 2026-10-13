package service

import (
	"context"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
)

// Repository defines the data access methods the service needs.
// The real implementation is *repository.AuthRepository.
// Tests can provide a mock that satisfies this interface.
type Repository interface {
	CreateUser(ctx context.Context, name, email, passwordHash string) (model.User, error)
	GetUserByEmail(ctx context.Context, email string) (model.User, string, error)
	GetUserByID(ctx context.Context, id int64) (model.User, error)
	UpdateProfile(ctx context.Context, id int64, name, image string) (model.User, error)
	CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error
	GetSessionIdentity(ctx context.Context, tokenHash string) (model.User, model.Session, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	DeleteAllSessions(ctx context.Context, userID int64) error
	PruneExpiredSessions(ctx context.Context) error
	ListPlatformRoleKeys(ctx context.Context, userID int64) ([]string, error)
	CreatePlatformAssignment(ctx context.Context, userID int64, roleKey string, createdBy int64) error
	InsertAuditLog(ctx context.Context, actorID int64, action, resource, result, requestID, ip string, detail map[string]any) error
	SeedManagedRoles(ctx context.Context) error

	// Admin console (IAM-09)
	ListRoles(ctx context.Context) ([]repository.IAMRole, error)
	ListAssignments(ctx context.Context, principalID int64, roleKey string, limit, offset int) ([]repository.IAMAssignment, error)
	GetAssignment(ctx context.Context, id int64) (repository.IAMAssignment, error)
	CreateAssignment(ctx context.Context, a repository.IAMAssignment) (repository.IAMAssignment, error)
	DeleteAssignment(ctx context.Context, id int64) error
	ListUsers(ctx context.Context, search, role, status string, limit, offset int) ([]repository.AdminUser, int, error)
	SetUserDisabled(ctx context.Context, id int64, disabled bool) (repository.AdminUser, error)
	ListAuditLog(ctx context.Context, actorID int64, action string, limit, offset int) ([]repository.AuditEntry, error)
	ListUsersByIDs(ctx context.Context, ids []int64) ([]repository.AdminUser, error)

	// Organizations (Phase 5 / ORG-01)
	ListOrgRoleEntries(ctx context.Context, userID int64) ([]string, error)
	ListOrganizations(ctx context.Context, limit, offset int) ([]repository.Organization, int, error)
	CreateOrganization(ctx context.Context, name, slug, kind string, ownerUserID int64) (repository.Organization, error)
	OrganizationExists(ctx context.Context, id int64) (bool, error)
}
