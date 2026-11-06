CREATE TABLE IF NOT EXISTS class_video_uploads (
 id text PRIMARY KEY,
 -- Retain unfinished sessions after class deletion so the expiry worker can remove their files.
 class_id bigint NOT NULL,
 user_id bigint NOT NULL,
 title text NOT NULL,
 audience text NOT NULL,
 viewer_ids jsonb NOT NULL,
 storage_key text NOT NULL,
 content_type text NOT NULL,
 size_bytes bigint NOT NULL CHECK(size_bytes > 0),
 received bigint NOT NULL DEFAULT 0,
 video_id bigint,
 expires_at timestamptz NOT NULL DEFAULT now() + interval '24 hours'
);
CREATE INDEX IF NOT EXISTS class_video_uploads_expiry ON class_video_uploads(expires_at);
