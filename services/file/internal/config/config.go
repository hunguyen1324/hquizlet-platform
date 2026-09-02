package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port        string
	DatabaseURL string
	Storage     StorageConfig
}

type StorageConfig struct {
	Provider        string // minio | s3 | r2
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey  string
	PublicBaseURL   string
	PathStyle       bool
	PresignTTLMins  int
}

func Load() Config {
	return Config{
		Port:        env("PORT", "8086"),
		DatabaseURL: env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"),
		Storage: StorageConfig{
			Provider:       env("STORAGE_PROVIDER", "minio"),
			Endpoint:       env("STORAGE_ENDPOINT", "http://localhost:9000"),
			Region:         env("STORAGE_REGION", "us-east-1"),
			Bucket:         env("STORAGE_BUCKET", "hquizlet"),
			AccessKeyID:    env("STORAGE_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: env("STORAGE_SECRET_KEY", "minioadmin"),
			PublicBaseURL:  env("STORAGE_PUBLIC_BASE_URL", "http://localhost:9000/hquizlet"),
			PathStyle:      envBool("STORAGE_PATH_STYLE", true),
			PresignTTLMins: envInt("STORAGE_PRESIGN_TTL_MINS", 15),
		},
	}
}

func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Storage.Provider == "" {
		return fmt.Errorf("STORAGE_PROVIDER is required")
	}
	switch c.Storage.Provider {
	case "minio", "s3", "r2":
	default:
		return fmt.Errorf("unknown STORAGE_PROVIDER: %s (must be minio, s3, or r2)", c.Storage.Provider)
	}
	if c.Storage.Bucket == "" {
		return fmt.Errorf("STORAGE_BUCKET is required")
	}
	if c.Storage.AccessKeyID == "" {
		return fmt.Errorf("STORAGE_ACCESS_KEY is required")
	}
	if c.Storage.SecretAccessKey == "" {
		return fmt.Errorf("STORAGE_SECRET_KEY is required")
	}
	return nil
}

func (c Config) PresignTTL() time.Duration {
	return time.Duration(c.Storage.PresignTTLMins) * time.Minute
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(strings.ToLower(v))
	if err != nil {
		return fallback
	}
	return b
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
