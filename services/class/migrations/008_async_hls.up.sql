ALTER TABLE class_videos ADD COLUMN IF NOT EXISTS processing_state TEXT NOT NULL DEFAULT 'ready';
ALTER TABLE class_videos ADD COLUMN IF NOT EXISTS hls_output_key TEXT;
ALTER TABLE class_videos DROP CONSTRAINT IF EXISTS class_videos_source_check;
ALTER TABLE class_videos ADD CONSTRAINT class_videos_source_check CHECK (
 (youtube_id IS NOT NULL AND storage_key IS NULL) OR
 (youtube_id IS NULL AND storage_key IS NOT NULL AND content_type IN ('video/mp4','video/webm','application/vnd.apple.mpegurl') AND size_bytes > 0)
);
CREATE OR REPLACE FUNCTION queue_class_video_deletion() RETURNS TRIGGER AS $$
BEGIN
 IF OLD.storage_key IS NOT NULL THEN
  INSERT INTO class_video_deletions(storage_key) VALUES(OLD.storage_key) ON CONFLICT DO NOTHING;
 END IF;
 IF OLD.hls_output_key IS NOT NULL THEN
  INSERT INTO class_video_deletions(storage_key) VALUES(OLD.hls_output_key) ON CONFLICT DO NOTHING;
 END IF;
 RETURN OLD;
END;
$$ LANGUAGE plpgsql;
