CREATE TABLE IF NOT EXISTS manga_volumes (
 id BIGSERIAL PRIMARY KEY, owner_user_id BIGINT NOT NULL, source_uuid VARCHAR(100) NOT NULL,
 title VARCHAR(200) NOT NULL, volume VARCHAR(200) NOT NULL DEFAULT '',
 visibility TEXT NOT NULL DEFAULT 'private' CHECK(visibility IN ('private','shared')),
 status TEXT NOT NULL DEFAULT 'importing' CHECK(status IN ('importing','ready')),
 page_count INTEGER NOT NULL CHECK(page_count BETWEEN 1 AND 1000),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(owner_user_id,source_uuid)
);
CREATE TABLE IF NOT EXISTS manga_pages (
 volume_id BIGINT NOT NULL REFERENCES manga_volumes(id) ON DELETE CASCADE,
 page_index INTEGER NOT NULL CHECK(page_index >= 0), image_key VARCHAR(80) NOT NULL UNIQUE,
 image_type VARCHAR(30) NOT NULL, size_bytes BIGINT NOT NULL,
 width INTEGER NOT NULL, height INTEGER NOT NULL, blocks JSONB NOT NULL,
 original_blocks JSONB NOT NULL, revision INTEGER NOT NULL DEFAULT 1,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY(volume_id,page_index)
);
CREATE TABLE IF NOT EXISTS manga_ocr_history (
 id BIGSERIAL PRIMARY KEY, volume_id BIGINT NOT NULL, page_index INTEGER NOT NULL,
 revision INTEGER NOT NULL, blocks JSONB NOT NULL, changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 FOREIGN KEY(volume_id,page_index) REFERENCES manga_pages(volume_id,page_index) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS manga_image_gc(image_key VARCHAR(80) PRIMARY KEY, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW());
CREATE INDEX IF NOT EXISTS manga_volumes_owner_idx ON manga_volumes(owner_user_id,id);
