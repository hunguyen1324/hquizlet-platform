// Package migration runs SQL migrations for the study service.
// Each migration is idempotent (uses IF NOT EXISTS / DO $$ guards)
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
// appended; never edit existing entries once applied to any shared environment.
var migrations = []string{
	// 001 – core tables (fresh DB)
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

	// 002 – add user_id if study_sets existed before it was introduced
	`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'study_sets' AND column_name = 'user_id'
		) THEN
			ALTER TABLE study_sets ADD COLUMN user_id BIGINT NOT NULL DEFAULT 0;
		END IF;
	END $$`,

	// 003 – add updated_at to study_sets if missing
	`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'study_sets' AND column_name = 'updated_at'
		) THEN
			ALTER TABLE study_sets ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
		END IF;
	END $$`,

	// 004 – add updated_at to flashcards if missing
	`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'flashcards' AND column_name = 'updated_at'
		) THEN
			ALTER TABLE flashcards ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
		END IF;
	END $$`,

	// 005 – index for fast flashcard lookups by study set
	`CREATE INDEX IF NOT EXISTS flashcards_study_set_id_idx ON flashcards(study_set_id)`,

	// 006 – index for fast user study-set listing (requires user_id column – safe after 002)
	`CREATE INDEX IF NOT EXISTS study_sets_user_id_idx ON study_sets(user_id)`,

	// 007 – seed a demo record for dev/CI convenience (harmless on prod)
	`INSERT INTO study_sets (user_id, title, description)
	 SELECT 0, 'Go + Rust migration basics', 'First demo study set stored in PostgreSQL'
	 WHERE NOT EXISTS (SELECT 1 FROM study_sets LIMIT 1)`,

	// 008 – add position column to flashcards for ordered display
	`DO $$ BEGIN
		IF NOT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'flashcards' AND column_name = 'position'
		) THEN
			ALTER TABLE flashcards ADD COLUMN position INT NOT NULL DEFAULT 0;
		END IF;
	END $$`,

	// 009 – starred_flashcards table (user-level, cross-set starring)
	`CREATE TABLE IF NOT EXISTS starred_flashcards (
		user_id     BIGINT NOT NULL,
		flashcard_id BIGINT NOT NULL REFERENCES flashcards(id) ON DELETE CASCADE,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (user_id, flashcard_id)
	)`,

	// 010 – folders table
	`CREATE TABLE IF NOT EXISTS folders (
		id          BIGSERIAL   PRIMARY KEY,
		user_id     BIGINT      NOT NULL,
		title       TEXT        NOT NULL CHECK (btrim(title) <> ''),
		description TEXT        NOT NULL DEFAULT '',
		created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
		updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
	)`,
	// Upgrade Phase 2/early Folder schemas that used `name` instead of `title`.
	`ALTER TABLE folders ADD COLUMN IF NOT EXISTS title TEXT`,
	`DO $$
	BEGIN
		IF EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = 'folders' AND column_name = 'name'
		) THEN
			UPDATE folders SET title = name WHERE title IS NULL;
			ALTER TABLE folders ALTER COLUMN name DROP NOT NULL;
		END IF;
	END $$`,
	`ALTER TABLE folders ALTER COLUMN title SET NOT NULL`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'folders_title_not_blank') THEN
			ALTER TABLE folders ADD CONSTRAINT folders_title_not_blank CHECK (btrim(title) <> '');
		END IF;
	END $$`,

	// 011 – folder_to_study_sets join table
	`CREATE TABLE IF NOT EXISTS folder_to_study_sets (
		folder_id    BIGINT NOT NULL REFERENCES folders(id) ON DELETE CASCADE,
		study_set_id BIGINT NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE,
		added_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
		PRIMARY KEY (folder_id, study_set_id)
	)`,

	// 012 – indexes for folder queries
	`CREATE INDEX IF NOT EXISTS folders_user_id_idx ON folders(user_id)`,
	`CREATE INDEX IF NOT EXISTS folder_to_study_sets_study_set_id_idx ON folder_to_study_sets(study_set_id)`,
	`CREATE INDEX IF NOT EXISTS starred_flashcards_user_id_idx ON starred_flashcards(user_id)`,

	// 013 – enable trigram search before creating the trigram index
	`CREATE EXTENSION IF NOT EXISTS pg_trgm`,

	// 014 – full-text search index on study_sets title
	`CREATE INDEX IF NOT EXISTS study_sets_title_trgm_idx ON study_sets USING gin(title gin_trgm_ops)`,

	// 015 – learning_sessions: one row per completed (or in-progress) learning session.
	// idempotency_key is unique per user to allow safe client retries.
	`CREATE TABLE IF NOT EXISTS learning_sessions (
		id              BIGSERIAL    PRIMARY KEY,
		user_id         BIGINT       NOT NULL,
		study_set_id    BIGINT       NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE,
		mode            TEXT         NOT NULL CHECK (mode IN ('flashcards','learn','test','match')),
		score           INT          NOT NULL CHECK (score >= 0),
		total           INT          NOT NULL CHECK (total >= 0 AND total <= 100),
		started_at      TIMESTAMPTZ  NOT NULL,
		completed_at    TIMESTAMPTZ,
		idempotency_key TEXT         NOT NULL,
		created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
		CONSTRAINT learning_sessions_score_lte_total CHECK (score <= total),
		CONSTRAINT learning_sessions_idempotency_key_uq UNIQUE (user_id, idempotency_key)
	)`,

	// 016 – learning_card_results: per-card results within a session.
	// flashcard_id must belong to the session's study_set – enforced at service layer.
	`CREATE TABLE IF NOT EXISTS learning_card_results (
		id               BIGSERIAL   PRIMARY KEY,
		session_id       BIGINT      NOT NULL REFERENCES learning_sessions(id) ON DELETE CASCADE,
		flashcard_id     BIGINT      NOT NULL REFERENCES flashcards(id) ON DELETE CASCADE,
		correct          BOOLEAN     NOT NULL,
		attempts         INT         NOT NULL CHECK (attempts BETWEEN 1 AND 100),
		response_time_ms INT         CHECK (response_time_ms IS NULL OR response_time_ms >= 0)
	)`,

	// 017 – indexes for progress queries
	`CREATE INDEX IF NOT EXISTS learning_sessions_user_set_created_idx
		ON learning_sessions(user_id, study_set_id, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS learning_sessions_session_id_idx
			ON learning_card_results(session_id)`,

	// 018 – optional flashcard learning metadata
	`ALTER TABLE flashcards ADD COLUMN IF NOT EXISTS image_url TEXT`,
	`ALTER TABLE flashcards ADD COLUMN IF NOT EXISTS example_sentence TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE flashcards ADD COLUMN IF NOT EXISTS hint_explanation TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE flashcards ADD COLUMN IF NOT EXISTS synonyms TEXT NOT NULL DEFAULT ''`,

	// ── Phase 10 Migrations ──────────────────────────────────────────────────────

	// 019 – content_type, term_language, definition_language, visibility on study_sets
	`ALTER TABLE study_sets ADD COLUMN IF NOT EXISTS content_type TEXT NOT NULL DEFAULT 'flashcard'`,
	`ALTER TABLE study_sets ADD COLUMN IF NOT EXISTS term_language TEXT NOT NULL DEFAULT 'en-US'`,
	`ALTER TABLE study_sets ADD COLUMN IF NOT EXISTS definition_language TEXT NOT NULL DEFAULT 'en-US'`,
	`ALTER TABLE study_sets ADD COLUMN IF NOT EXISTS visibility TEXT NOT NULL DEFAULT 'public'`,
	`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'study_sets_content_type_check') THEN
			ALTER TABLE study_sets ADD CONSTRAINT study_sets_content_type_check CHECK (content_type IN ('flashcard','quiz','grammar'));
		END IF;
	END $$`,
	`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'study_sets_visibility_check') THEN
			ALTER TABLE study_sets ADD CONSTRAINT study_sets_visibility_check CHECK (visibility IN ('public','private'));
		END IF;
	END $$`,

	// 020 – quiz_question table
	`CREATE TABLE IF NOT EXISTS quiz_question (
		id                BIGSERIAL PRIMARY KEY,
		study_set_id      BIGINT NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE,
		position          INT NOT NULL DEFAULT 0,
		question_text     TEXT NOT NULL DEFAULT '',
		question_type     TEXT NOT NULL DEFAULT 'multiple_choice'
			CHECK (question_type IN ('multiple_choice','true_false','written','paragraph','sorting')),
		correct_answer    TEXT,
		time_in_seconds   INT,
		audio_url         TEXT,
		answer_explanation TEXT,
		paragraph_text    TEXT,
		sub_questions     JSONB,
		tags              TEXT[] NOT NULL DEFAULT '{}'
	)`,
	`CREATE INDEX IF NOT EXISTS quiz_question_study_set_id_idx ON quiz_question(study_set_id)`,

	// 021 – quiz_question_option table
	`CREATE TABLE IF NOT EXISTS quiz_question_option (
		id          BIGSERIAL PRIMARY KEY,
		question_id BIGINT NOT NULL REFERENCES quiz_question(id) ON DELETE CASCADE,
		text        TEXT NOT NULL DEFAULT '',
		position    INT NOT NULL DEFAULT 0,
		is_correct  BOOLEAN NOT NULL DEFAULT false
	)`,
	`CREATE INDEX IF NOT EXISTS quiz_question_option_question_id_idx ON quiz_question_option(question_id)`,
}
