package migration

func init() {
	migrations = append(migrations,
		// Immutable question content, shared by every session of the same quiz
		// version. Previously each session copied the whole quiz into its state.
		`CREATE TABLE IF NOT EXISTS protected_quiz_snapshots (
 id TEXT PRIMARY KEY,
 study_set_id BIGINT NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE,
 part INT NOT NULL DEFAULT 0,
 total INT NOT NULL DEFAULT 0,
 payload JSONB NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now())`,
		`CREATE INDEX IF NOT EXISTS protected_quiz_snapshots_set_idx ON protected_quiz_snapshots(study_set_id)`,
		// Lets cleanup find unreferenced snapshots without decoding session state.
		`ALTER TABLE protected_quiz_sessions ADD COLUMN IF NOT EXISTS snapshot_id TEXT`,
		`CREATE INDEX IF NOT EXISTS protected_quiz_sessions_snapshot_idx ON protected_quiz_sessions(snapshot_id)`,
	)
}
