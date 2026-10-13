ALTER TABLE class_videos ALTER COLUMN youtube_id DROP NOT NULL;
ALTER TABLE class_videos ADD COLUMN IF NOT EXISTS storage_key TEXT;
ALTER TABLE class_videos ADD COLUMN IF NOT EXISTS content_type TEXT;
ALTER TABLE class_videos ADD COLUMN IF NOT EXISTS size_bytes BIGINT;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'class_videos_source_check') THEN
    ALTER TABLE class_videos ADD CONSTRAINT class_videos_source_check CHECK (
      (youtube_id IS NOT NULL AND storage_key IS NULL) OR
      (youtube_id IS NULL AND storage_key IS NOT NULL AND content_type IN ('video/mp4','video/webm') AND size_bytes > 0)
    );
  END IF;
END $$;

-- Preserve cleanup work even when a class deletion cascades its video metadata.
CREATE TABLE IF NOT EXISTS class_video_deletions (
  storage_key TEXT PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE OR REPLACE FUNCTION queue_class_video_deletion() RETURNS TRIGGER AS $$
BEGIN
  IF OLD.storage_key IS NOT NULL THEN
    INSERT INTO class_video_deletions(storage_key) VALUES(OLD.storage_key) ON CONFLICT DO NOTHING;
  END IF;
  RETURN OLD;
END;
$$ LANGUAGE plpgsql;
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'class_video_deletion_queue' AND tgrelid = 'class_videos'::regclass) THEN
    CREATE TRIGGER class_video_deletion_queue AFTER DELETE ON class_videos
      FOR EACH ROW EXECUTE FUNCTION queue_class_video_deletion();
  END IF;
END $$;
