-- Phase 6: Live Session Answers table
-- Dev 2 - [P6-DB-01]

CREATE TABLE IF NOT EXISTS live_session_answers (
  id              BIGSERIAL PRIMARY KEY,
  live_session_id BIGINT NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
  participant_id  UUID NOT NULL REFERENCES live_session_participants(id) ON DELETE CASCADE,
  question_index  INTEGER NOT NULL,
  flashcard_id    BIGINT NOT NULL,
  submitted_answer JSONB NOT NULL,
  is_correct      BOOLEAN NOT NULL,
  score_awarded   INTEGER NOT NULL,
  response_time_ms INTEGER NOT NULL,
  idempotency_key VARCHAR(128) NOT NULL,
  submitted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT uq_answer_session_participant_question
    UNIQUE (live_session_id, participant_id, question_index),
  CONSTRAINT uq_answer_participant_idempotency
    UNIQUE (participant_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_answers_session
  ON live_session_answers (live_session_id);
