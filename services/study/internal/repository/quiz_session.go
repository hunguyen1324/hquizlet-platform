package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"time"
)

var ErrQuizRateLimit = errors.New("quiz rate limit exceeded")
var ErrQuizSessionLimit = errors.New("too many active quiz sessions")

type QuizSessions interface {
	Recent(context.Context, int64, int64) ([]string, error)
	Create(context.Context, model.QuizSession) error
	Update(context.Context, string, int64, int64, func(*model.QuizSession) error) error
	Allow(context.Context, int64, string, int, time.Duration) error
}

type QuizSessionRepository struct{ db *sql.DB }

func NewQuizSessionRepository(db *sql.DB) *QuizSessionRepository {
	return &QuizSessionRepository{db: db}
}

// A database counter is shared by all replicas. No per-process bypass.
func (r *QuizSessionRepository) Allow(ctx context.Context, uid int64, bucket string, max int, window time.Duration) error {
	return r.AllowN(ctx, uid, bucket, 1, max, window)
}

// AllowN atomically charges n units (e.g. cards delivered) against a fixed
// window shared by all replicas. Requests that exceed the budget are rejected.
func (r *QuizSessionRepository) AllowN(ctx context.Context, uid int64, bucket string, n, max int, window time.Duration) error {
	if n < 1 {
		n = 1
	}
	var hits int
	seconds := int64(window / time.Second)
	err := r.db.QueryRowContext(ctx, `INSERT INTO protected_quiz_limits(user_id,bucket,window_start,hits)
 VALUES($1,$2,floor(extract(epoch from now())/$3)::bigint,$4)
 ON CONFLICT(user_id,bucket) DO UPDATE SET
 window_start=EXCLUDED.window_start,
 hits=CASE WHEN protected_quiz_limits.window_start=EXCLUDED.window_start THEN protected_quiz_limits.hits+EXCLUDED.hits ELSE EXCLUDED.hits END
 RETURNING hits`, uid, bucket, seconds, n).Scan(&hits)
	if err != nil {
		return err
	}
	if hits > max {
		return ErrQuizRateLimit
	}
	return nil
}
func (r *QuizSessionRepository) Create(ctx context.Context, s model.QuizSession) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Serialize creation by user, including requests handled by other replicas.
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, s.UserID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM protected_quiz_sessions WHERE user_id=$1 AND expires_at<now()`, s.UserID); err != nil {
		return err
	}
	var active int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM protected_quiz_sessions WHERE user_id=$1 AND active_until>now()`, s.UserID).Scan(&active); err != nil {
		return err
	}
	if active >= 2 {
		return ErrQuizSessionLimit
	}
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	activeUntil := s.ExpiresAt
	if s.Deadline != nil {
		activeUntil = *s.Deadline
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO protected_quiz_sessions(id,user_id,study_set_id,state,expires_at,active_until) VALUES($1,$2,$3,$4,$5,$6)`, s.ID, s.UserID, s.StudySetID, data, s.ExpiresAt, activeUntil)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (r *QuizSessionRepository) Update(ctx context.Context, id string, uid, setID int64, fn func(*model.QuizSession) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var data []byte
	err = tx.QueryRowContext(ctx, `SELECT state FROM protected_quiz_sessions WHERE id=$1 AND user_id=$2 AND study_set_id=$3 AND expires_at>now() FOR UPDATE`, id, uid, setID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var s model.QuizSession
	if err = json.Unmarshal(data, &s); err != nil {
		return err
	}
	if err = fn(&s); err != nil {
		return err
	}
	data, err = json.Marshal(s)
	if err != nil {
		return err
	}
	activeUntil := s.ExpiresAt
	if s.Deadline != nil {
		activeUntil = *s.Deadline
	}
	if s.SubmittedAt != nil {
		activeUntil = *s.SubmittedAt
	}
	_, err = tx.ExecContext(ctx, `UPDATE protected_quiz_sessions SET state=$2,active_until=$3 WHERE id=$1`, id, data, activeUntil)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *QuizSessionRepository) Recent(ctx context.Context, uid, setID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM protected_quiz_sessions WHERE user_id=$1 AND study_set_id=$2 AND expires_at>now() ORDER BY created_at DESC LIMIT 20`, uid, setID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
