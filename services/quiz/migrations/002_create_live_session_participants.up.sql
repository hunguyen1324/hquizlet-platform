-- Phase 6: Live Session Participants table
-- Dev 2 - [P6-DB-01]

CREATE TABLE IF NOT EXISTS live_session_participants (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  live_session_id BIGINT NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
  user_id         BIGINT,
  display_name    VARCHAR(40) NOT NULL,
  token_hash      CHAR(64) NOT NULL UNIQUE,
  total_score     INTEGER NOT NULL DEFAULT 0,
  correct_count   INTEGER NOT NULL DEFAULT 0,
  total_response_time_ms BIGINT NOT NULL DEFAULT 0,
  joined_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  left_at         TIMESTAMPTZ,

  CONSTRAINT uq_participant_session_name UNIQUE (live_session_id, display_name)
);

CREATE INDEX IF NOT EXISTS idx_participants_session
  ON live_session_participants (live_session_id);
