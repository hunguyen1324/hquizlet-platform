CREATE TABLE IF NOT EXISTS media_items (
 id BIGSERIAL PRIMARY KEY,
 owner_user_id BIGINT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('video','post','playlist')),
 title VARCHAR(200) NOT NULL,
 body TEXT NOT NULL DEFAULT '',
 source TEXT NOT NULL DEFAULT '',
 source_url TEXT NOT NULL DEFAULT '',
 embed_url TEXT NOT NULL DEFAULT '',
 thumbnail_url TEXT NOT NULL DEFAULT '',
 storage_key TEXT,
 content_type TEXT NOT NULL DEFAULT '',
 visibility TEXT NOT NULL DEFAULT 'public' CHECK(visibility IN ('public','private','selected')),
 viewer_ids JSONB NOT NULL DEFAULT '[]',
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS media_items_feed_idx ON media_items(kind,created_at DESC,id DESC);
CREATE INDEX IF NOT EXISTS media_items_owner_idx ON media_items(owner_user_id,id);
CREATE TABLE IF NOT EXISTS media_playlist_items (
 playlist_id BIGINT NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
 video_id BIGINT NOT NULL REFERENCES media_items(id) ON DELETE CASCADE,
 position INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(playlist_id,video_id)
);
