ALTER TABLE manga_volumes ADD COLUMN IF NOT EXISTS class_id BIGINT REFERENCES classes(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS manga_volumes_class_updated_idx ON manga_volumes(class_id,updated_at DESC,id DESC);
ALTER TABLE manga_volumes DROP CONSTRAINT IF EXISTS manga_volumes_owner_user_id_source_uuid_key;
CREATE UNIQUE INDEX IF NOT EXISTS manga_volumes_scope_source_idx ON manga_volumes(owner_user_id,source_uuid,COALESCE(class_id,0));
CREATE OR REPLACE FUNCTION queue_deleted_manga_images() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 INSERT INTO manga_image_gc(image_key) SELECT image_key FROM manga_pages WHERE volume_id=OLD.id ON CONFLICT DO NOTHING;
 RETURN OLD;
END $$;
DROP TRIGGER IF EXISTS manga_volume_image_cleanup ON manga_volumes;
CREATE TRIGGER manga_volume_image_cleanup BEFORE DELETE ON manga_volumes FOR EACH ROW EXECUTE FUNCTION queue_deleted_manga_images();
