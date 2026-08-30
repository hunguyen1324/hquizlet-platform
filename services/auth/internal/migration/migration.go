package migration

import "database/sql"

// Run applies all auth migrations idempotently.
// Safe to call on an existing DB:
//   - CREATE TABLE IF NOT EXISTS for new tables.
//   - ALTER TABLE ADD COLUMN IF NOT EXISTS for columns that may be missing on old DBs.
func Run(db *sql.DB) error {
	steps := []string{
		// --- users table ---
		`CREATE TABLE IF NOT EXISTS users (
			id            BIGSERIAL PRIMARY KEY,
			name          TEXT        NOT NULL,
			email         TEXT        NOT NULL UNIQUE,
			password_hash TEXT        NOT NULL,
			image         TEXT        NOT NULL DEFAULT '',
			role          TEXT        NOT NULL DEFAULT 'user',
			created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,

		// Backfill columns missing on DBs created before image/role were added.
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS image TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS role  TEXT NOT NULL DEFAULT 'user'`,

		// --- sessions table ---
		`CREATE TABLE IF NOT EXISTS sessions (
			id         BIGSERIAL PRIMARY KEY,
			user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT        NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`,

		`CREATE INDEX IF NOT EXISTS sessions_token_hash_idx ON sessions(token_hash)`,
		`CREATE INDEX IF NOT EXISTS sessions_user_id_idx    ON sessions(user_id)`,
	}

	for _, step := range steps {
		if _, err := db.Exec(step); err != nil {
			return err
		}
	}
	return nil
}
