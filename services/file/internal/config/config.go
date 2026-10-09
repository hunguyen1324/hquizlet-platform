package config

import (
	"fmt"
	"net/url"
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
	SecretAccessKey string
	PublicBaseURL   string
	PathStyle       bool
	PresignTTLMins  int
}

func Load() Config {
	c := Config{
		Port:        env("PORT", "8086"),
		DatabaseURL: env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"),
		Storage: StorageConfig{
			Provider:        env("STORAGE_PROVIDER", "minio"),
			Endpoint:        env("STORAGE_ENDPOINT", "http://localhost:9000"),
			Region:          env("STORAGE_REGION", "us-east-1"),
			Bucket:          env("STORAGE_BUCKET", "hquizlet"),
			AccessKeyID:     env("STORAGE_ACCESS_KEY", "minioadmin"),
			SecretAccessKey: env("STORAGE_SECRET_KEY", "minioadmin"),
			PublicBaseURL:   env("STORAGE_PUBLIC_BASE_URL", "http://localhost:9000/hquizlet"),
			PathStyle:       envBool("STORAGE_PATH_STYLE", true),
			PresignTTLMins:  envInt("STORAGE_PRESIGN_TTL_MINS", 15),
		},
	}
	if c.Storage.Provider == "r2" {
		// R2-specific variables take precedence over generic local-storage defaults.
		accountID := os.Getenv("R2_ACCOUNT_ID")
		c.Storage.Endpoint = env("R2_ENDPOINT", "")
		if c.Storage.Endpoint == "" && accountID != "" {
			c.Storage.Endpoint = "https://" + accountID + ".r2.cloudflarestorage.com"
		}
		if c.Storage.Endpoint == "" {
			c.Storage.Endpoint = os.Getenv("STORAGE_ENDPOINT")
		}
		c.Storage.Region = "auto"
		c.Storage.Bucket = env("R2_BUCKET_NAME", os.Getenv("STORAGE_BUCKET"))
		c.Storage.AccessKeyID = env("R2_ACCESS_KEY_ID", os.Getenv("STORAGE_ACCESS_KEY"))
		c.Storage.SecretAccessKey = env("R2_SECRET_ACCESS_KEY", os.Getenv("STORAGE_SECRET_KEY"))
		c.Storage.PublicBaseURL = strings.TrimRight(env("R2_PUBLIC_URL", os.Getenv("STORAGE_PUBLIC_BASE_URL")), "/")
		c.Storage.PathStyle = true
	}
	return c
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
	if c.Storage.Provider == "r2" {
		if c.Storage.PublicBaseURL == "" {
			return fmt.Errorf("R2_PUBLIC_URL is required for file service public image assets")
		}
		u, err := url.Parse(c.Storage.Endpoint)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return fmt.Errorf("R2 requires a valid HTTPS S3 endpoint (set R2_ACCOUNT_ID or R2_ENDPOINT)")
		}
	}
	if c.Storage.PresignTTLMins < 1 || c.Storage.PresignTTLMins > 10080 {
		return fmt.Errorf("STORAGE_PRESIGN_TTL_MINS must be between 1 and 10080")
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
