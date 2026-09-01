// Package repository – progress.go implements LearningProgress for PostgreSQL.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"math"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// LearningProgressRepository implements the LearningProgress interface.
type LearningProgressRepository struct {
	db *sql.DB
}

// NewLearningProgressRepository returns a new LearningProgressRepository.
func NewLearningProgressRepository(db *sql.DB) *LearningProgressRepository {
	return &LearningProgressRepository{db: db}
}

// Save inserts a learning session and all card results in one DB transaction.
// On any error, the entire transaction is rolled back.
// Returns ErrDuplicateIdempotencyKey when (user_id, idempotency_key) already exists.
func (r *LearningProgressRepository) Save(
	ctx context.Context,
	userID, studySetID int64,
	in model.SaveProgressInput,
) (model.LearningSession, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.LearningSession{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var session model.LearningSession
	err = tx.QueryRowContext(ctx,
		`INSERT INTO learning_sessions
			(user_id, study_set_id, mode, score, total,
			 started_at, completed_at, idempotency_key)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 RETURNING id, user_id, study_set_id, mode, score, total,
		           started_at, completed_at, idempotency_key, created_at`,
		userID, studySetID,
		string(in.Mode), in.Score, in.Total,
		in.StartedAt, in.CompletedAt, in.IdempotencyKey,
	).Scan(
		&session.ID, &session.UserID, &session.StudySetID,
		&session.Mode, &session.Score, &session.Total,
		&session.StartedAt, &session.CompletedAt,
		&session.IdempotencyKey, &session.CreatedAt,
	)
	if err != nil {
		if isDuplicateKeyError(err) {
			return model.LearningSession{}, ErrDuplicateIdempotencyKey
		}
		return model.LearningSession{}, err
	}

	// Insert card results in a batch; still within the same transaction.
	// Service layer guarantees len(in.CardResults) <= 100 before this point.
	for _, cr := range in.CardResults {
		var result model.LearningCardResult
		err = tx.QueryRowContext(ctx,
			`INSERT INTO learning_card_results
				(session_id, flashcard_id, correct, attempts, response_time_ms)
			 VALUES ($1,$2,$3,$4,$5)
			 RETURNING id, session_id, flashcard_id, correct, attempts, response_time_ms`,
			session.ID, cr.FlashcardID, cr.Correct, cr.Attempts, cr.ResponseTimeMs,
		).Scan(
			&result.ID, &result.SessionID, &result.FlashcardID,
			&result.Correct, &result.Attempts, &result.ResponseTimeMs,
		)
		if err != nil {
			return model.LearningSession{}, err
		}
		session.CardResults = append(session.CardResults, result)
	}

	if err = tx.Commit(); err != nil {
		return model.LearningSession{}, err
	}
	return session, nil
}

// GetSummary returns aggregate stats and a paginated history for one user+study set.
// Only sessions owned by userID for studySetID are returned.
func (r *LearningProgressRepository) GetSummary(
	ctx context.Context,
	userID, studySetID int64,
	f model.ProgressFilter,
) (model.ProgressSummary, error) {
	summary := model.ProgressSummary{
		StudySetID: studySetID,
		Page:       f.Page,
		PerPage:    f.PerPage,
	}

	// Aggregate counts and best score in one query.
	var bestScore sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*), MAX(score)
		 FROM learning_sessions
		 WHERE user_id = $1 AND study_set_id = $2`,
		userID, studySetID,
	).Scan(&summary.TotalSessions, &bestScore)
	if err != nil {
		return summary, err
	}
	if bestScore.Valid {
		v := int(bestScore.Int64)
		summary.BestScore = &v
	}

	// Last mode used.
	var lastMode sql.NullString
	err = r.db.QueryRowContext(ctx,
		`SELECT mode FROM learning_sessions
		 WHERE user_id = $1 AND study_set_id = $2
		 ORDER BY created_at DESC LIMIT 1`,
		userID, studySetID,
	).Scan(&lastMode)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return summary, err
	}
	if lastMode.Valid {
		m := model.LearningMode(lastMode.String)
		summary.LastMode = &m
	}

	// Total pages.
	if f.PerPage > 0 && summary.TotalSessions > 0 {
		summary.TotalPages = int(math.Ceil(float64(summary.TotalSessions) / float64(f.PerPage)))
	}

	// Paginated history (no card results for list view – keeps payload small).
	offset := (f.Page - 1) * f.PerPage
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, study_set_id, mode, score, total,
		        started_at, completed_at, idempotency_key, created_at
		 FROM learning_sessions
		 WHERE user_id = $1 AND study_set_id = $2
		 ORDER BY created_at DESC
		 LIMIT $3 OFFSET $4`,
		userID, studySetID, f.PerPage, offset,
	)
	if err != nil {
		return summary, err
	}
	defer rows.Close()

	for rows.Next() {
		var s model.LearningSession
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.StudySetID,
			&s.Mode, &s.Score, &s.Total,
			&s.StartedAt, &s.CompletedAt,
			&s.IdempotencyKey, &s.CreatedAt,
		); err != nil {
			return summary, err
		}
		summary.History = append(summary.History, s)
	}
	return summary, rows.Err()
}

// GetLatestByMode returns the most-recent session for the given mode.
// Returns ErrNotFound when no session exists for that mode.
func (r *LearningProgressRepository) GetLatestByMode(
	ctx context.Context,
	userID, studySetID int64,
	mode model.LearningMode,
) (model.LearningSession, error) {
	var s model.LearningSession
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, study_set_id, mode, score, total,
		        started_at, completed_at, idempotency_key, created_at
		 FROM learning_sessions
		 WHERE user_id = $1 AND study_set_id = $2 AND mode = $3
		 ORDER BY created_at DESC LIMIT 1`,
		userID, studySetID, string(mode),
	).Scan(
		&s.ID, &s.UserID, &s.StudySetID,
		&s.Mode, &s.Score, &s.Total,
		&s.StartedAt, &s.CompletedAt,
		&s.IdempotencyKey, &s.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}

// isDuplicateKeyError detects PostgreSQL unique constraint violations.
// We match on SQLSTATE 23505 via the error string since lib/pq is not imported here.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// pq driver: "pq: duplicate key value violates unique constraint ..."
	// pgx driver: "ERROR: duplicate key value violates unique constraint ..."
	for _, sub := range []string{"23505", "duplicate key"} {
		if containsStr(msg, sub) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
