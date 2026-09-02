-- Phase 6: Live Sessions table
-- Dev 2 - [P6-DB-01]

CREATE TABLE IF NOT EXISTS live_sessions (
  id              BIGSERIAL PRIMARY KEY,
  code            VARCHAR(8) NOT NULL UNIQUE,
  host_user_id    BIGINT NOT NULL,
  study_set_id    BIGINT NOT NULL,
  status          VARCHAR(24) NOT NULL DEFAULT 'LOBBY',
  seed            BIGINT NOT NULL,
  question_count  INTEGER NOT NULL,
  question_duration_ms INTEGER NOT NULL,
  current_question_index INTEGER,
  state_version   BIGINT NOT NULL DEFAULT 1,
  question_snapshot JSONB NOT NULL DEFAULT '[]',
  started_at      TIMESTAMPTZ,
  ended_at        TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CONSTRAINT chk_live_sessions_status CHECK (
    status IN ('LOBBY','QUESTION_OPEN','QUESTION_CLOSED','LEADERBOARD','ENDED')
  ),
  CONSTRAINT chk_live_sessions_question_count CHECK (question_count > 0),
  CONSTRAINT chk_live_sessions_duration CHECK (question_duration_ms >= 5000 AND question_duration_ms <= 120000)
);

CREATE INDEX IF NOT EXISTS idx_live_sessions_host_created
  ON live_sessions (host_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_live_sessions_status_updated
  ON live_sessions (status, updated_at);
