-- Phase 6: Live Event Outbox table rollback
-- Dev 2 - [P6-DB-01]

DROP INDEX IF EXISTS idx_outbox_pending;
DROP TABLE IF EXISTS live_event_outbox;
