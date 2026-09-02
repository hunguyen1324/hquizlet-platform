package migration

// 018 – Phase 9: Add image_url to flashcards and thumbnail_url to study_sets.
// These columns store the URL of uploaded images via the File service.
func init() {
	migrations = append(migrations,
		// Add image_url to flashcards
		`DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'flashcards' AND column_name = 'image_url'
			) THEN
				ALTER TABLE flashcards ADD COLUMN image_url TEXT;
			END IF;
		END $$`,

		// Add thumbnail_url to study_sets
		`DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'study_sets' AND column_name = 'thumbnail_url'
			) THEN
				ALTER TABLE study_sets ADD COLUMN thumbnail_url TEXT;
			END IF;
		END $$`,
	)
}
