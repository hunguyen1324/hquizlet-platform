-- 004: Create activity_events table
CREATE TABLE IF NOT EXISTS activity_events (
  id            BIGSERIAL     PRIMARY KEY,
  user_id       BIGINT        NOT NULL,
  event_type    VARCHAR(64)   NOT NULL,
  entity_type   VARCHAR(32)   NOT NULL,
  entity_id     BIGINT,
  class_id      BIGINT        REFERENCES classes(id) ON DELETE SET NULL,
  metadata      JSONB,
  occurred_at   TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS activity_events_user_id_occurred_at_idx
  ON activity_events(user_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS activity_events_class_id_idx
  ON activity_events(class_id);
