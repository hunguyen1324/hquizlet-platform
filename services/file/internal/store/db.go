package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Open connects to PostgreSQL and retries until reachable.
func Open(dsn string) *sql.DB {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("[file] db open: %v", err)
	}
	ctx := context.Background()
	for attempt := 1; attempt <= 20; attempt++ {
		if err := db.PingContext(ctx); err == nil {
			log.Printf("[file] connected to database")
			return db
		}
		log.Printf("[file] waiting for postgres, attempt %d/20", attempt)
		time.Sleep(time.Second)
	}
	log.Fatal("[file] postgres unreachable after 20 attempts")
	return db
}

// RunMigrations executes all migration SQL statements.
func RunMigrations(db *sql.DB) error {
	for i, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	return nil
}

var migrations = []string{
	// 001 – uploaded_file table
	`CREATE TABLE IF NOT EXISTS uploaded_file (
		"id"           uuid      PRIMARY KEY DEFAULT gen_random_uuid(),
		"user_id"      bigint    NOT NULL,
		"upload_type"  text      NOT NULL
			CHECK ("upload_type" IN ('avatar', 'flashcard_image', 'study_set_thumbnail')),
		"storage_key"  text      NOT NULL UNIQUE,
		"filename"     text      NOT NULL,
		"content_type" text      NOT NULL,
		"size_bytes"   bigint    NOT NULL,
		"public_url"   text,
		"status"       text      NOT NULL DEFAULT 'pending'
			CHECK ("status" IN ('pending', 'active', 'deleted')),
		"created_at"   timestamptz NOT NULL DEFAULT now(),
		"confirmed_at" timestamptz,
		"deleted_at"   timestamptz
	)`,
	// 002 – indexes
	`CREATE INDEX IF NOT EXISTS idx_uf_user_status ON uploaded_file (user_id, status)`,
	`CREATE INDEX IF NOT EXISTS idx_uf_user_type_active ON uploaded_file (user_id, upload_type) WHERE status = 'active'`,
	`CREATE INDEX IF NOT EXISTS idx_uf_pending_created ON uploaded_file (created_at) WHERE status = 'pending'`,
}
