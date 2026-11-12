package migration

func init() {
	migrations = append(migrations, `CREATE TABLE IF NOT EXISTS spaced_repetition (
 user_id BIGINT NOT NULL,
 flashcard_id BIGINT NOT NULL REFERENCES flashcards(id) ON DELETE CASCADE,
 repetitions INT NOT NULL DEFAULT 0,
 interval_days INT NOT NULL DEFAULT 0,
 due_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 reviewed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 PRIMARY KEY(user_id, flashcard_id))`)
}
