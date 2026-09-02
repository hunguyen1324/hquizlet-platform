-- 005: Create class_event_outbox table for NATS JetStream outbox pattern
CREATE TABLE IF NOT EXISTS class_event_outbox (
  event_id       UUID          PRIMARY KEY,
  aggregate_id   BIGINT        NOT NULL,
  subject        VARCHAR(120)  NOT NULL,
  event_version  INTEGER       NOT NULL,
  payload        JSONB         NOT NULL,
  occurred_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
  published_at   TIMESTAMPTZ,
  attempts       INTEGER       NOT NULL DEFAULT 0,
  last_error     TEXT
);

-- Partial index for the outbox poller: only scan unpublished events
CREATE INDEX IF NOT EXISTS class_event_outbox_unpublished_idx
  ON class_event_outbox(occurred_at)
  WHERE published_at IS NULL;
