// Package repository – admin.go holds read-only queries for the admin console (ADM-06).
package repository

import (
	"context"
	"database/sql"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

// AdminStore is the platform-wide read surface used by the admin console.
type AdminStore interface {
	ListAllClasses(ctx context.Context, search string, limit, offset int) ([]*model.AdminClassRow, int, error)
	Exists(ctx context.Context, classID int64) (bool, error)
}

// AdminRepository implements AdminStore on PostgreSQL.
type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// ListAllClasses returns every class (not just the caller's) with its member and content counts.
func (r *AdminRepository) ListAllClasses(ctx context.Context, search string, limit, offset int) ([]*model.AdminClassRow, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	if offset < 0 {
		offset = 0
	}
	search = strings.TrimSpace(search)
	var total int
	if err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM classes c WHERE ($1 = '' OR c.name ILIKE '%'||$1||'%')`, search,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT c.id, c.name, c.owner_user_id,
		        (SELECT COUNT(*) FROM class_members m WHERE m.class_id = c.id),
		        (SELECT COUNT(*) FROM class_study_sets s WHERE s.class_id = c.id),
		        c.created_at
		 FROM classes c
		 WHERE ($1 = '' OR c.name ILIKE '%'||$1||'%')
		 ORDER BY c.created_at DESC, c.id DESC
		 LIMIT $2 OFFSET $3`,
		search, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]*model.AdminClassRow, 0, limit)
	for rows.Next() {
		var row model.AdminClassRow
		if err := rows.Scan(&row.ID, &row.Name, &row.OwnerUserID, &row.MemberCount, &row.StudySetCount, &row.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, &row)
	}
	return out, total, rows.Err()
}

// Exists reports whether a class id is present (used to validate admin actions).
func (r *AdminRepository) Exists(ctx context.Context, classID int64) (bool, error) {
	var one int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM classes WHERE id = $1`, classID).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

var _ AdminStore = (*AdminRepository)(nil)
