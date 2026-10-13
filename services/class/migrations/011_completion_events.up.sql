ALTER TABLE activity_events ADD COLUMN IF NOT EXISTS source_event_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS activity_events_source_class_unique ON activity_events(source_event_id,class_id);
