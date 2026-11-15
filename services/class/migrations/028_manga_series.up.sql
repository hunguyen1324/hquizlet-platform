CREATE TABLE IF NOT EXISTS manga_series (
 id BIGSERIAL PRIMARY KEY,
 owner_user_id BIGINT NOT NULL,
 class_id BIGINT REFERENCES classes(id) ON DELETE CASCADE,
 title VARCHAR(200) NOT NULL CHECK(btrim(title)<>''),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS manga_series_scope_title_idx ON manga_series(owner_user_id,(COALESCE(class_id,0)),(lower(btrim(title))));
ALTER TABLE manga_volumes ADD COLUMN IF NOT EXISTS series_id BIGINT REFERENCES manga_series(id);
-- Preserve volume IDs, images, OCR, permissions and reader progress during backfill.
INSERT INTO manga_series(owner_user_id,class_id,title)
 SELECT owner_user_id,class_id,min(title) FROM manga_volumes WHERE series_id IS NULL GROUP BY owner_user_id,class_id,lower(btrim(title)) ON CONFLICT DO NOTHING;
UPDATE manga_volumes v SET series_id=s.id FROM manga_series s WHERE v.series_id IS NULL AND v.owner_user_id=s.owner_user_id AND COALESCE(v.class_id,0)=COALESCE(s.class_id,0) AND lower(btrim(v.title))=lower(btrim(s.title));
CREATE INDEX IF NOT EXISTS manga_volumes_series_idx ON manga_volumes(series_id,id);
