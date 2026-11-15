package migration_test

import (
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/migration"
	"testing"
)

func TestKanjiIndexTracksCardLifecycle(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()
	if err := migration.Run(db); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var setID, cardID int64
	if err = tx.QueryRow(`INSERT INTO study_sets(user_id,title) VALUES(123456,'Kanji index lifecycle') RETURNING id`).Scan(&setID); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow(`INSERT INTO flashcards(study_set_id,term,definition) VALUES($1,'時時𠮷','reading') RETURNING id`, setID).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	count := func(want int) {
		t.Helper()
		var n int
		if e := tx.QueryRow(`SELECT count(*) FROM kanji_word_index WHERE flashcard_id=$1`, cardID).Scan(&n); e != nil || n != want {
			t.Fatalf("index count=%d want=%d err=%v", n, want, e)
		}
	}
	count(2)
	if _, err = tx.Exec(`UPDATE flashcards SET term='改造' WHERE id=$1`, cardID); err != nil {
		t.Fatal(err)
	}
	var stale int
	if err = tx.QueryRow(`SELECT count(*) FROM kanji_word_index WHERE flashcard_id=$1 AND char='時'`, cardID).Scan(&stale); err != nil || stale != 0 {
		t.Fatalf("old links retained %d %v", stale, err)
	}
	count(2)
	if _, err = tx.Exec(`DELETE FROM flashcards WHERE id=$1`, cardID); err != nil {
		t.Fatal(err)
	}
	count(0)
}
