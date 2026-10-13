// Package repository – stats.go implements the dashboard statistics queries.
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// StudyStatsStore is the read-only surface the dashboard service depends on.
type StudyStatsStore interface {
	DailySeries(ctx context.Context, userID int64, from, to time.Time) ([]model.StudyDayPoint, error)
	Totals(ctx context.Context, userID int64, from time.Time) (sessionsTotal, sessionsInRange, cardsInRange, correctInRange int, err error)
	DueCards(ctx context.Context, userID int64) (int, error)
	RecentSessions(ctx context.Context, userID int64, limit int) ([]model.RecentSession, error)
	ContinueLearning(ctx context.Context, userID int64, limit int) ([]model.ContinueItem, error)
	OwnedSets(ctx context.Context, userID int64, limit int) ([]model.OwnedSetStat, error)
	ProgressByUsersAndSets(ctx context.Context, studySetIDs, userIDs []int64) ([]model.ProgressSummaryRow, error)
}

// StatsRepository implements StudyStatsStore on PostgreSQL.
type StatsRepository struct {
	db *sql.DB
}

func NewStatsRepository(db *sql.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// DailySeries fills every day in [from, to] so the chart has no gaps.
func (r *StatsRepository) DailySeries(ctx context.Context, userID int64, from, to time.Time) ([]model.StudyDayPoint, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT to_char(d.day, 'YYYY-MM-DD') AS day,
		       COUNT(DISTINCT ls.id) AS sessions,
		       COUNT(lcr.id) AS cards,
		       COALESCE(SUM(CASE WHEN lcr.correct THEN 1 ELSE 0 END), 0) AS correct
		FROM generate_series($2::date, $3::date, interval '1 day') AS d(day)
		LEFT JOIN learning_sessions ls
		       ON ls.user_id = $1 AND ls.created_at::date = d.day
		LEFT JOIN learning_card_results lcr ON lcr.session_id = ls.id
		GROUP BY d.day
		ORDER BY d.day`,
		userID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.StudyDayPoint, 0, 32)
	for rows.Next() {
		var p model.StudyDayPoint
		if err := rows.Scan(&p.Date, &p.Sessions, &p.Cards, &p.Correct); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Totals returns all-time session count plus range-scoped session/card totals.
func (r *StatsRepository) Totals(ctx context.Context, userID int64, from time.Time) (int, int, int, int, error) {
	var sessionsTotal, sessionsInRange, cardsInRange, correctInRange int
	err := r.db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM learning_sessions WHERE user_id = $1),
		  COUNT(DISTINCT ls.id),
		  COUNT(lcr.id),
		  COALESCE(SUM(CASE WHEN lcr.correct THEN 1 ELSE 0 END), 0)
		FROM learning_sessions ls
		LEFT JOIN learning_card_results lcr ON lcr.session_id = ls.id
		WHERE ls.user_id = $1 AND ls.created_at >= $2`,
		userID, from,
	).Scan(&sessionsTotal, &sessionsInRange, &cardsInRange, &correctInRange)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return sessionsTotal, sessionsInRange, cardsInRange, correctInRange, nil
}

// DueCards counts cards whose most recent attempt was incorrect ("thẻ cần ôn").
func (r *StatsRepository) DueCards(ctx context.Context, userID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT DISTINCT ON (lcr.flashcard_id) lcr.correct
			FROM learning_card_results lcr
			JOIN learning_sessions ls ON ls.id = lcr.session_id
			WHERE ls.user_id = $1
			ORDER BY lcr.flashcard_id, ls.created_at DESC
		) latest
		WHERE latest.correct = false`,
		userID,
	).Scan(&n)
	return n, err
}

// RecentSessions returns the caller's latest completed sessions with set titles.
func (r *StatsRepository) RecentSessions(ctx context.Context, userID int64, limit int) ([]model.RecentSession, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ls.study_set_id, COALESCE(ss.title, ''), ls.mode, ls.score, ls.total,
		       to_char(ls.completed_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'), to_char(ls.created_at, 'YYYY-MM-DD"T"HH24:MI:SSZ')
		FROM learning_sessions ls
		LEFT JOIN study_sets ss ON ss.id = ls.study_set_id
		WHERE ls.user_id = $1
		ORDER BY ls.created_at DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.RecentSession, 0, limit)
	for rows.Next() {
		var s model.RecentSession
		var completed sql.NullString
		if err := rows.Scan(&s.StudySetID, &s.StudySetTitle, &s.Mode, &s.Score, &s.Total, &completed, &s.CreatedAt); err != nil {
			return nil, err
		}
		if completed.Valid {
			s.CompletedAt = &completed.String
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ContinueLearning returns sets the caller has already studied, most recent first.
func (r *StatsRepository) ContinueLearning(ctx context.Context, userID int64, limit int) ([]model.ContinueItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ls.study_set_id, COALESCE(ss.title, ''),
		       (ARRAY_AGG(ls.mode ORDER BY ls.created_at DESC))[1] AS last_mode,
		       COUNT(*) AS sessions,
		       MAX(ls.score) AS best_score,
		       to_char(MAX(ls.created_at), 'YYYY-MM-DD"T"HH24:MI:SSZ') AS last_played
		FROM learning_sessions ls
		LEFT JOIN study_sets ss ON ss.id = ls.study_set_id
		WHERE ls.user_id = $1
		GROUP BY ls.study_set_id, ss.title
		ORDER BY MAX(ls.created_at) DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ContinueItem, 0, limit)
	for rows.Next() {
		var c model.ContinueItem
		if err := rows.Scan(&c.StudySetID, &c.Title, &c.LastMode, &c.Sessions, &c.BestScore, &c.LastPlayedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// OwnedSets summarises usage of sets owned by the caller (teacher dashboard).
func (r *StatsRepository) OwnedSets(ctx context.Context, userID int64, limit int) ([]model.OwnedSetStat, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT ss.id, ss.title,
		       COUNT(ls.id) AS sessions,
		       COUNT(DISTINCT ls.user_id) AS learners,
		       COALESCE(ROUND(AVG(CASE WHEN ls.total > 0 THEN ls.score::numeric * 100 / ls.total END)), 0)::int AS avg_pct
		FROM study_sets ss
		LEFT JOIN learning_sessions ls ON ls.study_set_id = ss.id
		WHERE ss.user_id = $1
		GROUP BY ss.id, ss.title
		ORDER BY sessions DESC, ss.id DESC
		LIMIT $2`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.OwnedSetStat, 0, limit)
	for rows.Next() {
		var s model.OwnedSetStat
		if err := rows.Scan(&s.StudySetID, &s.Title, &s.Sessions, &s.Learners, &s.AvgScorePct); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ProgressByUsersAndSets aggregates progress for a batch of (user, study set) pairs.
// "Completed" means the learner reached at least 80% on one session of that set.
func (r *StatsRepository) ProgressByUsersAndSets(ctx context.Context, studySetIDs, userIDs []int64) ([]model.ProgressSummaryRow, error) {
	if len(studySetIDs) == 0 || len(userIDs) == 0 {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT user_id, study_set_id,
		       COUNT(*)::int AS sessions,
		       MAX(score)::int AS best_score,
		       COALESCE(ROUND(AVG(CASE WHEN total > 0 THEN score::numeric * 100 / total END)), 0)::int AS avg_pct,
		       BOOL_OR(total > 0 AND score::numeric * 100 / total >= 80) AS completed,
		       to_char(MAX(created_at), 'YYYY-MM-DD"T"HH24:MI:SSZ') AS last_activity
		FROM learning_sessions
		WHERE study_set_id = ANY($1) AND user_id = ANY($2)
		GROUP BY user_id, study_set_id`,
		studySetIDs, userIDs,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ProgressSummaryRow, 0, len(studySetIDs))
	for rows.Next() {
		var row model.ProgressSummaryRow
		var last sql.NullString
		if err := rows.Scan(&row.UserID, &row.StudySetID, &row.Sessions, &row.BestScore, &row.AvgScorePct, &row.Completed, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			row.LastActivity = &last.String
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ensure the concrete type satisfies the interface at build time.
var _ StudyStatsStore = (*StatsRepository)(nil)
