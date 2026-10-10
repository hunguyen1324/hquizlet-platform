CREATE TABLE IF NOT EXISTS class_videos (
  id BIGSERIAL PRIMARY KEY,
  class_id BIGINT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
  title VARCHAR(200) NOT NULL,
  youtube_id VARCHAR(11) NOT NULL,
  audience VARCHAR(16) NOT NULL CHECK (audience IN ('all', 'selected')),
  viewer_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS class_videos_class_idx ON class_videos(class_id);
