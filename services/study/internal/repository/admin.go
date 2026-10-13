// Package repository – admin.go implements the platform-wide study-set listing (ADM-06).
package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// AdminStore is the platform-wide read surface used by the admin console.
type AdminStore interface {
	ListAllStudySets(ctx context.Context, search string, limit, offset int) ([]*model.AdminStudySetRow, int, error)
}

// AdminRepository implements AdminStore on PostgreSQL.
type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// ListAllStudySets returns every study set regardless of owner, with content counts.
func (r *AdminRepository) ListAllStudySets(ctx context.Context, search string, limit, offset int) ([]*model.AdminStudySetRow, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	search = strings.TrimSpace(search)

	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM study_sets s WHERE ($1 = '' OR s.title ILIKE '%'||$1||'%')`, search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT s.id, s.title, s.user_id,
		        (SELECT COUNT(*) FROM flashcards f WHERE f.study_set_id = s.id),
		        (SELECT COUNT(*) FROM quiz_question q WHERE q.study_set_id = s.id),
		        s.updated_at, s.created_at, s.moderation_hidden
		 FROM study_sets s
		 WHERE ($1 = '' OR s.title ILIKE '%'||$1||'%')
		 ORDER BY s.created_at DESC, s.id DESC
		 LIMIT $2 OFFSET $3`,
		search, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]*model.AdminStudySetRow, 0, limit)
	for rows.Next() {
		var row model.AdminStudySetRow
		if err := rows.Scan(&row.ID, &row.Title, &row.OwnerUserID, &row.CardCount, &row.QuizCount, &row.UpdatedAt, &row.CreatedAt, &row.Hidden); err != nil {
			return nil, 0, err
		}
		out = append(out, &row)
	}
	return out, total, rows.Err()
}

var _ AdminStore = (*AdminRepository)(nil)
