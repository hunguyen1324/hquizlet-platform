// Package repository – dashboard.go holds queries backing the class dashboard.
package repository

import (
	"context"
	"database/sql"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

// DashboardStore is the small read/write surface the class dashboard needs on top of the
// existing class/member/study-set stores.
type DashboardStore interface {
	LeaderboardEnabled(ctx context.Context, classID int64) (bool, error)
	SetLeaderboard(ctx context.Context, classID int64, enabled bool) error
	ListClassActivity(ctx context.Context, classID int64, limit int) ([]*model.ActivityEvent, error)
}

// DashboardRepository implements DashboardStore on PostgreSQL.
type DashboardRepository struct {
	db *sql.DB
}

func NewDashboardRepository(db *sql.DB) *DashboardRepository {
	return &DashboardRepository{db: db}
}

// LeaderboardEnabled reports whether students of the class may see the leaderboard.
func (r *DashboardRepository) LeaderboardEnabled(ctx context.Context, classID int64) (bool, error) {
	var enabled bool
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(show_leaderboard, false) FROM classes WHERE id = $1`, classID,
	).Scan(&enabled)
	if err == sql.ErrNoRows {
		return false, ErrNotFound
	}
	return enabled, err
}

// SetLeaderboard toggles the per-class leaderboard visibility (owner/teacher only, enforced upstream).
func (r *DashboardRepository) SetLeaderboard(ctx context.Context, classID int64, enabled bool) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE classes SET show_leaderboard = $2, updated_at = now() WHERE id = $1`,
		classID, enabled,
	)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListClassActivity returns the most recent activity events for one class.
func (r *DashboardRepository) ListClassActivity(ctx context.Context, classID int64, limit int) ([]*model.ActivityEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, event_type, entity_type, entity_id, class_id, COALESCE(metadata,'{}'::jsonb), occurred_at
		 FROM activity_events WHERE class_id = $1 ORDER BY occurred_at DESC LIMIT $2`,
		classID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.ActivityEvent
	for rows.Next() {
		var e model.ActivityEvent
		if err := rows.Scan(&e.ID, &e.UserID, &e.EventType, &e.EntityType, &e.EntityID, &e.ClassID, &e.Metadata, &e.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}

var _ DashboardStore = (*DashboardRepository)(nil)
