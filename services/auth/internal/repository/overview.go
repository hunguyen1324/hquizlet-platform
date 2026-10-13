// Package repository – overview.go implements the admin console overview queries.
package repository

import (
	"context"
	"database/sql"
	"time"
)

// OverviewCounts holds the scalar KPIs for one time window plus the previous window.
type OverviewCounts struct {
	TotalUsers      int
	NewUsers        int
	NewUsersPrev    int
	ActiveUsers     int
	ActiveUsersPrev int
	Assignments     int
	AuditEvents     int
	DeniedAttempts  int
}

// OverviewPoint is one day of the admin time series.
type OverviewPoint struct {
	Date        string
	NewUsers    int
	ActiveUsers int
}

// OverviewStore is the read surface the admin overview service depends on.
type OverviewStore interface {
	Counts(ctx context.Context, from, prevFrom time.Time) (OverviewCounts, error)
	Series(ctx context.Context, from, to time.Time) ([]OverviewPoint, error)
}

// OverviewRepository implements OverviewStore on PostgreSQL.
type OverviewRepository struct {
	db *sql.DB
}

func NewOverviewRepository(db *sql.DB) *OverviewRepository {
	return &OverviewRepository{db: db}
}

// Counts returns all scalar KPIs in one round trip.
func (r *OverviewRepository) Counts(ctx context.Context, from, prevFrom time.Time) (OverviewCounts, error) {
	var c OverviewCounts
	err := r.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM users),
		  (SELECT COUNT(*) FROM users WHERE created_at >= $1),
		  (SELECT COUNT(*) FROM users WHERE created_at >= $2 AND created_at < $1),
		  (SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= $1),
		  (SELECT COUNT(DISTINCT user_id) FROM sessions WHERE created_at >= $2 AND created_at < $1),
		  (SELECT COUNT(*) FROM iam_assignments WHERE expires_at IS NULL OR expires_at>now()),
		  (SELECT COUNT(*) FROM audit_log WHERE at >= $1),
		  (SELECT COUNT(*) FROM audit_log WHERE at >= $1 AND result = 'deny')`,
		from, prevFrom,
	).Scan(&c.TotalUsers, &c.NewUsers, &c.NewUsersPrev, &c.ActiveUsers, &c.ActiveUsersPrev,
		&c.Assignments, &c.AuditEvents, &c.DeniedAttempts)
	return c, err
}

// Series returns a gap-filled daily time series for new and active users.
func (r *OverviewRepository) Series(ctx context.Context, from, to time.Time) ([]OverviewPoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT to_char(d.day, 'YYYY-MM-DD'),
		       COALESCE((SELECT value FROM metrics_daily m WHERE m.service='auth' AND m.name='newUsers' AND m.day=d.day),0),
		       COALESCE((SELECT value FROM metrics_daily m WHERE m.service='auth' AND m.name='activeUsers' AND m.day=d.day),0)
		FROM generate_series($1::date, $2::date, interval '1 day') AS d(day)
		ORDER BY d.day`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]OverviewPoint, 0, 32)
	for rows.Next() {
		var p OverviewPoint
		if err := rows.Scan(&p.Date, &p.NewUsers, &p.ActiveUsers); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

var _ OverviewStore = (*OverviewRepository)(nil)
