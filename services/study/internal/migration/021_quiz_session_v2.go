package migration

func init() {
	migrations = append(migrations,
		`ALTER TABLE protected_quiz_sessions ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ`,
		`UPDATE protected_quiz_sessions SET updated_at=created_at WHERE updated_at IS NULL`,
		// Per-quiz index: speeds up the 5-session quota check on (user_id, study_set_id).
		`CREATE INDEX IF NOT EXISTS protected_quiz_sessions_user_set_idx
         ON protected_quiz_sessions(user_id, study_set_id, active_until)`,
		// Soft-delete column: lets us mark a session deleted without immediately losing data.
		`ALTER TABLE protected_quiz_sessions ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
	)
}
