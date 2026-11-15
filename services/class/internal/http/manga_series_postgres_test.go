package http

import (
	"database/sql"
	"github.com/hunguyen1324/hquizlet-platform/services/class/migrations"
	"testing"
)

func testMangaSeriesBackfill(t *testing.T, db *sql.DB) {
	t.Helper()
	var first, second int64
	for i, dest := range []*int64{&first, &second} {
		if err := db.QueryRow(`INSERT INTO manga_volumes(owner_user_id,source_uuid,title,volume,page_count,status,visibility) VALUES(1001,$1,'Existing story',$2,1,'ready','private') RETURNING id`, "legacy-series-"+string(rune('a'+i)), string(rune('1'+i))).Scan(dest); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO manga_pages(volume_id,page_index,image_key,image_type,size_bytes,width,height,blocks,original_blocks,revision) VALUES($1,0,'existing-page.png','image/png',100,16,16,'[{"box":[0,0,10,10],"lines":["edited text"]}]','[{"box":[0,0,10,10],"lines":["original text"]}]',3)`, first); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := migrations.Run(db); err != nil {
			t.Fatal(err)
		}
	}
	var firstSeries, secondSeries int64
	if err := db.QueryRow(`SELECT series_id FROM manga_volumes WHERE id=$1`, first).Scan(&firstSeries); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT series_id FROM manga_volumes WHERE id=$1`, second).Scan(&secondSeries); err != nil {
		t.Fatal(err)
	}
	if firstSeries < 1 || firstSeries != secondSeries {
		t.Fatalf("existing volumes not grouped: %d %d", firstSeries, secondSeries)
	}
	var key, ocr string
	var revision int
	if err := db.QueryRow(`SELECT image_key,blocks->0->'lines'->>0,revision FROM manga_pages WHERE volume_id=$1`, first).Scan(&key, &ocr, &revision); err != nil {
		t.Fatal(err)
	}
	if key != "existing-page.png" || ocr != "edited text" || revision != 3 {
		t.Fatalf("migration changed existing page: %s %s %d", key, ocr, revision)
	}
}
