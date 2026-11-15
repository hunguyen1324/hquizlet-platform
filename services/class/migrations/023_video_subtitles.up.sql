CREATE TABLE IF NOT EXISTS class_video_subtitles (
 video_id BIGINT NOT NULL REFERENCES class_videos(id) ON DELETE CASCADE,
 kind TEXT NOT NULL CHECK(kind IN ('original','translation')),
 language VARCHAR(35) NOT NULL DEFAULT '',
 label VARCHAR(100) NOT NULL,
 cues JSONB NOT NULL CHECK(jsonb_typeof(cues)='array'),
 updated_by BIGINT NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(video_id,kind)
);
