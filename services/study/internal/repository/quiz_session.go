package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

var ErrQuizRateLimit = errors.New("quiz rate limit exceeded")
var ErrQuizSessionLimit = errors.New("too many active quiz sessions")

// MaxSessionsPerQuiz is the per-(user, quiz) cap for concurrently active sessions.
const MaxSessionsPerQuiz = 5

// QuizSessionMeta contains the display fields for the session management page.
type QuizSessionMeta struct {
	ID          string     `json:"id"`
	StudySetID  int64      `json:"studySetId"`
	Mode        string     `json:"mode"`
	Instant     bool       `json:"instant"`
	Layout      string     `json:"layout"`
	StartedAt   time.Time  `json:"startedAt"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	Deadline    *time.Time `json:"deadline,omitempty"`
	SubmittedAt *time.Time `json:"submittedAt,omitempty"`
	ActiveUntil time.Time  `json:"activeUntil"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	Answered    int        `json:"answered"`
	Total       int        `json:"total"`
}

type QuizSessions interface {
	Recent(context.Context, int64, int64) ([]string, error)
	Create(context.Context, model.QuizSession) error
	Update(context.Context, string, int64, int64, func(*model.QuizSession) error) error
	Allow(context.Context, int64, string, int, time.Duration) error

	// List returns session metadata for all non-deleted sessions of this user,
	// ordered by most-recently updated first.
	// If studySetID > 0, results are filtered to that quiz.
	List(ctx context.Context, userID, studySetID int64, page, perPage int) ([]QuizSessionMeta, int, error)

	// Delete soft-deletes a session owned by the given user.
	// Returns ErrNotFound if the session does not exist or belongs to another user.
	// Returns ErrNotFound (idempotent) if already deleted.
	Delete(ctx context.Context, id string, userID int64) error

	// CountActive counts non-deleted, non-expired sessions for (user, quiz).
	CountActive(ctx context.Context, userID, studySetID int64) (int, error)
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

// Create inserts a new session, enforcing the per-(user, quiz) limit of MaxSessionsPerQuiz.
// A pg_advisory_xact_lock on user_id prevents concurrent races across replicas.
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
	// Hard-delete expired sessions to keep the table tidy.
	if _, err = tx.ExecContext(ctx, `DELETE FROM protected_quiz_sessions WHERE user_id=$1 AND expires_at<now()`, s.UserID); err != nil {
		return err
	}
	// Count active sessions for this specific (user, study_set) pair.
	var active int
	if err = tx.QueryRowContext(ctx,
		`SELECT count(*) FROM protected_quiz_sessions
         WHERE user_id=$1 AND study_set_id=$2
           AND active_until>now() AND deleted_at IS NULL`,
		s.UserID, s.StudySetID).Scan(&active); err != nil {
		return err
	}
	if active >= MaxSessionsPerQuiz {
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
	_, err = tx.ExecContext(ctx,
		`INSERT INTO protected_quiz_sessions(id,user_id,study_set_id,state,expires_at,active_until)
         VALUES($1,$2,$3,$4,$5,$6)`,
		s.ID, s.UserID, s.StudySetID, data, s.ExpiresAt, activeUntil)
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
	err = tx.QueryRowContext(ctx,
		`SELECT state FROM protected_quiz_sessions
         WHERE id=$1 AND user_id=$2 AND study_set_id=$3
           AND expires_at>now() AND deleted_at IS NULL FOR UPDATE`,
		id, uid, setID).Scan(&data)
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
	_, err = tx.ExecContext(ctx,
		`UPDATE protected_quiz_sessions SET state=$2,active_until=$3,updated_at=now() WHERE id=$1`,
		id, data, activeUntil)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Recent returns session IDs for an unexpired, non-deleted quiz, most-recent first.
// Used by the summary endpoint so the player can resume a previous session.
func (r *QuizSessionRepository) Recent(ctx context.Context, uid, setID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id FROM protected_quiz_sessions
         WHERE user_id=$1 AND study_set_id=$2
           AND expires_at>now() AND deleted_at IS NULL
         ORDER BY created_at DESC LIMIT 20`,
		uid, setID)
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

// CountActive returns the number of active non-deleted sessions for (user, quiz).
func (r *QuizSessionRepository) CountActive(ctx context.Context, userID, studySetID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM protected_quiz_sessions
         WHERE user_id=$1 AND study_set_id=$2
           AND active_until>now() AND deleted_at IS NULL`,
		userID, studySetID).Scan(&n)
	return n, err
}

// List returns paginated session metadata for a user.  When studySetID>0 results
// are filtered to that quiz. Total is the total count without pagination.
func (r *QuizSessionRepository) List(ctx context.Context, userID, studySetID int64, page, perPage int) ([]QuizSessionMeta, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	setFilter := ""
	args := []any{userID}
	if studySetID > 0 {
		args = append(args, studySetID)
		setFilter = " AND study_set_id=$2"
	}

	countQ := `SELECT count(*) FROM protected_quiz_sessions WHERE user_id=$1` + setFilter + ` AND deleted_at IS NULL`
	var total int
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []QuizSessionMeta{}, 0, nil
	}

	// Append pagination args.
	args = append(args, perPage, offset)
	limitPos := len(args) - 1
	offsetPos := len(args)
	// Build query with correct param positions.
	listQ := `SELECT id, study_set_id, state, expires_at, active_until,
	              COALESCE(updated_at, created_at) AS updated_at
              FROM protected_quiz_sessions
              WHERE user_id=$1` + setFilter +
		` AND deleted_at IS NULL
              ORDER BY COALESCE(updated_at, created_at) DESC
              LIMIT $` + sessionItoa(limitPos) + ` OFFSET $` + sessionItoa(offsetPos)

	rows, err := r.db.QueryContext(ctx, listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var metas []QuizSessionMeta
	for rows.Next() {
		var m QuizSessionMeta
		var data []byte
		var activeUntil time.Time
		var updatedAt time.Time
		if err := rows.Scan(&m.ID, &m.StudySetID, &data, &m.ExpiresAt, &activeUntil, &updatedAt); err != nil {
			return nil, 0, err
		}
		m.UpdatedAt = updatedAt
		m.ActiveUntil = activeUntil
		var s model.QuizSession
		if err := json.Unmarshal(data, &s); err == nil {
			m.Mode = s.Mode
			m.Instant = s.Instant
			m.Layout = s.Layout
			m.StartedAt = s.StartedAt
			m.Deadline = s.Deadline
			m.SubmittedAt = s.SubmittedAt
			m.Total = len(s.Items)
			m.Answered = len(s.Answers)
		}
		metas = append(metas, m)
	}
	if metas == nil {
		metas = []QuizSessionMeta{}
	}
	return metas, total, rows.Err()
}

// Delete soft-deletes a session that belongs to the given user.
// Idempotent: returns nil if already deleted.
func (r *QuizSessionRepository) Delete(ctx context.Context, id string, userID int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE protected_quiz_sessions
         SET deleted_at=now(), active_until=now()
         WHERE id=$1 AND user_id=$2 AND deleted_at IS NULL`,
		id, userID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		// Either wrong owner or already deleted — verify ownership to distinguish.
		var exists bool
		if err2 := r.db.QueryRowContext(ctx,
			`SELECT true FROM protected_quiz_sessions WHERE id=$1 AND user_id=$2`,
			id, userID).Scan(&exists); err2 != nil {
			if errors.Is(err2, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err2
		}
		// Session exists but was already deleted — idempotent success.
	}
	return nil
}

// sessionItoa converts an int to its string representation without importing strconv.
func sessionItoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := make([]byte, 0, 10)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	return string(buf)
}
