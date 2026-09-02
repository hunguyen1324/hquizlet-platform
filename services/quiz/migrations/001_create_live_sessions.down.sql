-- Phase 6: Live Sessions table rollback
-- Dev 2 - [P6-DB-01]

DROP INDEX IF EXISTS idx_live_sessions_status_updated;
DROP INDEX IF EXISTS idx_live_sessions_host_created;
DROP TABLE IF EXISTS live_sessions;
