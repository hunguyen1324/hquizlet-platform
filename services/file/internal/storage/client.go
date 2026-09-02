package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/config"
)

// Storage is the unified interface for MinIO, S3, and R2.
// Swapping backends requires zero service code changes.
type Storage interface {
	// PresignPut generates a presigned URL for direct client PUT.
	PresignPut(ctx context.Context, key string, contentType string, ttl time.Duration) (url string, expiresAt time.Time, err error)

	// HeadObject verifies an object exists and returns its metadata.
	HeadObject(ctx context.Context, key string) (size int64, contentType string, err error)

	// PublicURL returns a publicly accessible URL for the object.
	PublicURL(ctx context.Context, key string, ttl time.Duration) (string, error)

	// DeleteObject removes an object from storage.
	DeleteObject(ctx context.Context, key string) error
}

// New creates a Storage implementation based on the provider config.
func New(cfg config.StorageConfig) (Storage, error) {
	switch cfg.Provider {
	case "minio":
		return newMinIO(cfg)
	case "s3", "r2":
		return newS3(cfg)
	default:
		return nil, fmt.Errorf("unknown storage provider: %s", cfg.Provider)
	}
}
