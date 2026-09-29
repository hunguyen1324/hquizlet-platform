package migration

func init() {
	migrations = append(migrations,
		`ALTER TABLE protected_quiz_audio ADD COLUMN IF NOT EXISTS source_url TEXT`,
		`ALTER TABLE protected_quiz_audio ADD COLUMN IF NOT EXISTS object_key TEXT`,
		`ALTER TABLE protected_quiz_audio ADD COLUMN IF NOT EXISTS backed_up_at TIMESTAMPTZ`,
	)
}
