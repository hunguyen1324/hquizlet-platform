package migration

import "database/sql"

// Run applies all auth migrations idempotently.
// Safe to call on a fresh DB or an existing DB from older versions.
// Each statement uses IF NOT EXISTS or DO $$ blocks so repeated runs are harmless.
func Run(db *sql.DB) error {
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

var migrations = []string{
	// 001 – create users table (fresh DB)
	`CREATE TABLE IF NOT EXISTS users (
		id            BIGSERIAL    PRIMARY KEY,
		name          TEXT         NOT NULL,
		email         TEXT         NOT NULL UNIQUE,
		password_hash TEXT         NOT NULL,
		image         TEXT         NOT NULL DEFAULT '',
		role          TEXT         NOT NULL DEFAULT 'user',
		created_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
	)`,

	// 002 – add image column if DB existed before it was introduced
	`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'image'
		) THEN
			ALTER TABLE users ADD COLUMN image TEXT NOT NULL DEFAULT '';
		END IF;
	END $$`,

	// 003 – add role column if DB existed before it was introduced
	`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'role'
		) THEN
			ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';
		END IF;
	END $$`,

	// 004 – create sessions table
	`CREATE TABLE IF NOT EXISTS sessions (
		id         BIGSERIAL    PRIMARY KEY,
		user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token_hash TEXT         NOT NULL UNIQUE,
		expires_at TIMESTAMPTZ  NOT NULL,
		created_at TIMESTAMPTZ  NOT NULL DEFAULT now()
	)`,

	// 005 – indexes
	`CREATE INDEX IF NOT EXISTS sessions_token_hash_idx ON sessions(token_hash)`,
	`CREATE INDEX IF NOT EXISTS sessions_user_id_idx    ON sessions(user_id)`,
}
