package http

import (
	"context"
	"database/sql"
)

// Series are scoped to a space. A guessed ID never grants access to another
// owner's private community series or to a different classroom.
func resolveMangaSeries(ctx context.Context, tx *sql.Tx, uid, classID, seriesID int64, title string) (int64, string, error) {
	if seriesID > 0 {
		err := tx.QueryRowContext(ctx, `SELECT s.id,s.title FROM manga_series s WHERE s.id=$1 AND COALESCE(s.class_id,0)=$2 AND (s.owner_user_id=$3 OR EXISTS(SELECT 1 FROM manga_volumes own WHERE own.series_id=s.id AND own.owner_user_id=$3) OR ($2>0 AND EXISTS(SELECT 1 FROM manga_volumes v WHERE v.series_id=s.id AND v.visibility='shared' AND v.status='ready')))`, seriesID, classID, uid).Scan(&seriesID, &title)
		return seriesID, title, err
	}
	err := tx.QueryRowContext(ctx, `INSERT INTO manga_series(owner_user_id,class_id,title) VALUES($1,NULLIF($2,0),$3) ON CONFLICT (owner_user_id,(COALESCE(class_id,0)),(lower(btrim(title)))) DO UPDATE SET title=manga_series.title RETURNING id,title`, uid, classID, title).Scan(&seriesID, &title)
	return seriesID, title, err
}
