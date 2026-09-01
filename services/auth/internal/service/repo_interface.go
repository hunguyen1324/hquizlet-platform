package service

import (
	"context"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
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
}
