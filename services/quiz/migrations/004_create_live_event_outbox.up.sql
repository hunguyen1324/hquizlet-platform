-- Phase 6: Live Event Outbox table
-- Dev 2 - [P6-DB-01]

CREATE TABLE IF NOT EXISTS live_event_outbox (
  event_id        UUID PRIMARY KEY,
  aggregate_id    BIGINT NOT NULL,
  subject         VARCHAR(120) NOT NULL,
  event_version   INTEGER NOT NULL DEFAULT 1,
  payload         JSONB NOT NULL,
  occurred_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  published_at    TIMESTAMPTZ,
  attempts        INTEGER NOT NULL DEFAULT 0,
  last_error      TEXT
);

-- Partial index for worker polling: only unpublished events, ordered by occurrence
CREATE INDEX IF NOT EXISTS idx_outbox_pending
  ON live_event_outbox (occurred_at)
  WHERE published_at IS NULL;
