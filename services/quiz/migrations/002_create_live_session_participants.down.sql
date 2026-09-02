-- Phase 6: Live Session Participants table rollback
-- Dev 2 - [P6-DB-01]

DROP INDEX IF EXISTS idx_participants_session;
DROP TABLE IF EXISTS live_session_participants;
