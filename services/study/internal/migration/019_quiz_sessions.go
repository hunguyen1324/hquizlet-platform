package migration

func init() {
	migrations = append(migrations,
		`CREATE TABLE IF NOT EXISTS protected_quiz_audio (id TEXT PRIMARY KEY, study_set_id BIGINT NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE, content_type TEXT NOT NULL, body BYTEA NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE TABLE IF NOT EXISTS protected_quiz_sessions (
 id TEXT PRIMARY KEY, user_id BIGINT NOT NULL, study_set_id BIGINT NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE,
 state JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), expires_at TIMESTAMPTZ NOT NULL,
 active_until TIMESTAMPTZ NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS protected_quiz_sessions_user_idx ON protected_quiz_sessions(user_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS protected_quiz_limits (
 user_id BIGINT NOT NULL, bucket TEXT NOT NULL, window_start BIGINT NOT NULL, hits INT NOT NULL,
 PRIMARY KEY(user_id, bucket))`,
	)
}
