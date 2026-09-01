// Package migration_test verifies that all migrations are idempotent and that
// the progress tables (015-017) are created with the expected schema.
//
// This is a unit-level structural test: it runs migrations against an in-process
// SQLite-compatible schema parser is NOT used; instead we test against a real
// PostgreSQL instance only when DATABASE_URL is set, skipping otherwise.
//
// To run locally:
//   DATABASE_URL=postgres://... go test ./internal/migration/...
package migration_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/migration"
)

func TestRun_IdempotentOnEmptyDB(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	// Run twice – should not error on second run (all statements use IF NOT EXISTS / DO $$).
	for i := 1; i <= 2; i++ {
		if err := migration.Run(db); err != nil {
			t.Fatalf("migration.Run() iteration %d: %v", i, err)
		}
	}
}

func TestRun_LearningSessionsTableExists(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if err := migration.Run(db); err != nil {
		t.Fatalf("migration.Run(): %v", err)
	}
	assertTableExists(t, db, "learning_sessions")
}

func TestRun_LearningCardResultsTableExists(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if err := migration.Run(db); err != nil {
		t.Fatalf("migration.Run(): %v", err)
	}
	assertTableExists(t, db, "learning_card_results")
}

func TestRun_LearningSessionsColumns(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if err := migration.Run(db); err != nil {
		t.Fatalf("migration.Run(): %v", err)
	}
	required := []string{
		"id", "user_id", "study_set_id", "mode", "score", "total",
		"started_at", "completed_at", "idempotency_key", "created_at",
	}
	for _, col := range required {
		assertColumnExists(t, db, "learning_sessions", col)
	}
}

func TestRun_LearningCardResultsColumns(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if err := migration.Run(db); err != nil {
		t.Fatalf("migration.Run(): %v", err)
	}
	required := []string{"id", "session_id", "flashcard_id", "correct", "attempts", "response_time_ms"}
	for _, col := range required {
		assertColumnExists(t, db, "learning_card_results", col)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping integration migration test")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}
	return db
}

func assertTableExists(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`, table,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("checking table %q: %v", table, err)
	}
	if !exists {
		t.Errorf("expected table %q to exist, but it does not", table)
	}
}

func assertColumnExists(t *testing.T, db *sql.DB, table, column string) {
	t.Helper()
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
		)`, table, column,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("checking column %q.%q: %v", table, column, err)
	}
	if !exists {
		t.Errorf("expected column %q in table %q, but it does not exist", column, table)
	}
}
