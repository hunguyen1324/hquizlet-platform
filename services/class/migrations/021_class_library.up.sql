ALTER TABLE class_videos ADD COLUMN IF NOT EXISTS category VARCHAR(100) NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS class_documents (
 id BIGSERIAL PRIMARY KEY,
 class_id BIGINT NOT NULL REFERENCES classes(id) ON DELETE CASCADE,
 name VARCHAR(255) NOT NULL,
 content_type TEXT NOT NULL,
 data BYTEA NOT NULL CHECK (octet_length(data) BETWEEN 1 AND 20971520),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS class_documents_class_idx ON class_documents(class_id, created_at DESC);
