// Package migration runs SQL migrations for the study service.
// Each migration is idempotent (uses IF NOT EXISTS / DO NOTHING guards)
// so it is safe to run on every startup or against an existing database.
package migration

import "database/sql"

// Run applies all study-service migrations in order.
func Run(db *sql.DB) error {
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

// migrations are ordered SQL statements. New migrations must always be
// appended; never edit existing entries once they have been applied to
// any shared environment.
var migrations = []string{
	// 001 – core tables
	`CREATE TABLE IF NOT EXISTS study_sets (
		id          BIGSERIAL    PRIMARY KEY,
		user_id     BIGINT       NOT NULL DEFAULT 0,
		title       TEXT         NOT NULL,
		description TEXT         NOT NULL DEFAULT '',
		created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
		updated_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
	)`,

	`CREATE TABLE IF NOT EXISTS flashcards (
		id           BIGSERIAL   PRIMARY KEY,
		study_set_id BIGINT      NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE,
		term         TEXT        NOT NULL,
		definition   TEXT        NOT NULL,
		starred      BOOLEAN     NOT NULL DEFAULT false,
		created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
	)`,

	// 002 – index for fast flashcard lookups by study set
	`CREATE INDEX IF NOT EXISTS flashcards_study_set_id_idx ON flashcards(study_set_id)`,

	// 003 – index for fast user study-set listing
	`CREATE INDEX IF NOT EXISTS study_sets_user_id_idx ON study_sets(user_id)`,

	// 004 – seed a demo record for dev/CI convenience (harmless on prod)
	`INSERT INTO study_sets (user_id, title, description)
	 SELECT 0, 'Go + Rust migration basics', 'First demo study set stored in PostgreSQL'
	 WHERE NOT EXISTS (SELECT 1 FROM study_sets LIMIT 1)`,
}
