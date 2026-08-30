package migration

import "database/sql"

// Run applies all auth migrations idempotently.
// Safe to call on an existing DB – uses IF NOT EXISTS / ADD COLUMN IF NOT EXISTS.
func Run(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            BIGSERIAL PRIMARY KEY,
			name          TEXT        NOT NULL,
			email         TEXT        NOT NULL UNIQUE,
			password_hash TEXT        NOT NULL,
			image         TEXT        NOT NULL DEFAULT '',
			role          TEXT        NOT NULL DEFAULT 'user',
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id         BIGSERIAL PRIMARY KEY,
			user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT        NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS sessions_token_hash_idx ON sessions(token_hash);
		CREATE INDEX IF NOT EXISTS sessions_user_id_idx    ON sessions(user_id);
	`)
	return err
}
