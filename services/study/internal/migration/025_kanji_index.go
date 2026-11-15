package migration

func init() {
	migrations = append(migrations,
		`CREATE TABLE IF NOT EXISTS kanji_word_index (
  char TEXT NOT NULL, flashcard_id BIGINT NOT NULL REFERENCES flashcards(id) ON DELETE CASCADE,
  study_set_id BIGINT NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE,
  PRIMARY KEY(char,flashcard_id)
 )`,
		`CREATE INDEX IF NOT EXISTS kanji_word_index_set_char_idx ON kanji_word_index(study_set_id,char,flashcard_id)`,
		`CREATE OR REPLACE FUNCTION sync_flashcard_kanji() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
  DELETE FROM kanji_word_index WHERE flashcard_id=NEW.id;
  INSERT INTO kanji_word_index(char,flashcard_id,study_set_id)
  SELECT DISTINCT c,NEW.id,NEW.study_set_id FROM regexp_split_to_table(NEW.term,'') c WHERE c COLLATE "C" ~ '[㐀-䶿一-鿿豈-﫿𠀀-𲎯]' ON CONFLICT DO NOTHING;
  RETURN NEW;
 END $$`,
		`DROP TRIGGER IF EXISTS flashcard_kanji_index ON flashcards`,
		`CREATE TRIGGER flashcard_kanji_index AFTER INSERT OR UPDATE OF term,study_set_id ON flashcards FOR EACH ROW EXECUTE FUNCTION sync_flashcard_kanji()`,
		// Existing cards are backfilled once. Subsequent writes are transactional with the card.
		`CREATE TABLE IF NOT EXISTS study_data_migrations(key TEXT PRIMARY KEY,applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`DO $$ BEGIN PERFORM pg_advisory_xact_lock(250025); IF NOT EXISTS(SELECT 1 FROM study_data_migrations WHERE key='kanji-word-index-v1') THEN
 LOCK TABLE flashcards IN SHARE ROW EXCLUSIVE MODE;
 INSERT INTO kanji_word_index(char,flashcard_id,study_set_id)
 SELECT DISTINCT c,f.id,f.study_set_id FROM flashcards f CROSS JOIN LATERAL regexp_split_to_table(f.term,'') c WHERE c COLLATE "C" ~ '[㐀-䶿一-鿿豈-﫿𠀀-𲎯]' ON CONFLICT DO NOTHING;
 INSERT INTO study_data_migrations(key) VALUES('kanji-word-index-v1');
 END IF; END $$`,
	)
}
