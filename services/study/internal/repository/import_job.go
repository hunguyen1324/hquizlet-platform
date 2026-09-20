package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// ImportJobRepository implements ImportJobs backed by PostgreSQL.
type ImportJobRepository struct {
	db *sql.DB
}

// NewImportJobRepository creates a new repository backed by db.
func NewImportJobRepository(db *sql.DB) *ImportJobRepository {
	return &ImportJobRepository{db: db}
}

// Create inserts a new import_jobs row with status=pending.
func (r *ImportJobRepository) Create(ctx context.Context, in model.CreateImportJobInput) (model.ImportJob, error) {
	const q = `
		INSERT INTO import_jobs (user_id, study_set_id, kind, status, total, imported, errors, error_msg)
		VALUES ($1, $2, $3, 'pending', 0, 0, '[]', '')
		RETURNING id, user_id, study_set_id, kind, status, total, imported, errors, error_msg, created_at, updated_at`

	row := r.db.QueryRowContext(ctx, q, in.UserID, in.StudySetID, string(in.Kind))
	return scanJob(row)
}

// Get fetches a single import job by ID.
func (r *ImportJobRepository) Get(ctx context.Context, id int64) (model.ImportJob, error) {
	const q = `
		SELECT id, user_id, study_set_id, kind, status, total, imported, errors, error_msg, created_at, updated_at
		FROM import_jobs WHERE id = $1`

	row := r.db.QueryRowContext(ctx, q, id)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return model.ImportJob{}, ErrNotFound
	}
	return job, err
}

// Update sets status, counters, and result fields for an existing job.
func (r *ImportJobRepository) Update(ctx context.Context, id int64, in model.UpdateImportJobInput) (model.ImportJob, error) {
	errsJSON, err := json.Marshal(in.Errors)
	if err != nil {
		return model.ImportJob{}, fmt.Errorf("marshal errors: %w", err)
	}

	const q = `
		UPDATE import_jobs
		SET status = $1, total = $2, imported = $3, errors = $4, error_msg = $5, updated_at = NOW()
		WHERE id = $6
		RETURNING id, user_id, study_set_id, kind, status, total, imported, errors, error_msg, created_at, updated_at`

	row := r.db.QueryRowContext(ctx, q, string(in.Status), in.Total, in.Imported, errsJSON, in.ErrorMsg, id)
	job, err := scanJob(row)
	if err == sql.ErrNoRows {
		return model.ImportJob{}, ErrNotFound
	}
	return job, err
}

// ListByUser returns the most-recent `limit` jobs for a user, newest-first.
func (r *ImportJobRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]model.ImportJob, error) {
	if limit <= 0 {
		limit = 20
	}
	const q = `
		SELECT id, user_id, study_set_id, kind, status, total, imported, errors, error_msg, created_at, updated_at
		FROM import_jobs
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []model.ImportJob
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

// scanJob scans a *sql.Row into an ImportJob.
func scanJob(row *sql.Row) (model.ImportJob, error) {
	var j model.ImportJob
	var errsJSON []byte
	err := row.Scan(
		&j.ID, &j.UserID, &j.StudySetID,
		&j.Kind, &j.Status,
		&j.Total, &j.Imported,
		&errsJSON, &j.ErrorMsg,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return j, err
	}
	if len(errsJSON) > 0 {
		_ = json.Unmarshal(errsJSON, &j.Errors)
	}
	return j, nil
}

// scanJobRow scans a *sql.Rows into an ImportJob.
func scanJobRow(rows *sql.Rows) (model.ImportJob, error) {
	var j model.ImportJob
	var errsJSON []byte
	err := rows.Scan(
		&j.ID, &j.UserID, &j.StudySetID,
		&j.Kind, &j.Status,
		&j.Total, &j.Imported,
		&errsJSON, &j.ErrorMsg,
		&j.CreatedAt, &j.UpdatedAt,
	)
	if err != nil {
		return j, err
	}
	if len(errsJSON) > 0 {
		_ = json.Unmarshal(errsJSON, &j.Errors)
	}
	return j, nil
}

// ---------------------------------------------------------------------------
// Paginated import job list with filtering
// ---------------------------------------------------------------------------

// ListWithFilter returns paginated import jobs for a user with optional filters.
// Filters: StudySetID (0 = all), Kind ("" = all), Status ("" = all).
func (r *ImportJobRepository) ListWithFilter(ctx context.Context, userID int64, f model.ImportJobFilter) (model.ImportJobListResult, error) {
	page, perPage := model.ClampPage(f.Page, f.PerPage, 100)
	offset := (page - 1) * perPage

	args := []any{userID}
	whereExtra := ""

	if f.StudySetID > 0 {
		args = append(args, f.StudySetID)
		whereExtra += " AND study_set_id = $" + itoa(len(args))
	}
	if f.Kind != "" {
		args = append(args, f.Kind)
		whereExtra += " AND kind = $" + itoa(len(args))
	}
	if f.Status != "" {
		args = append(args, f.Status)
		whereExtra += " AND status = $" + itoa(len(args))
	}

	var total int
	countQ := `SELECT COUNT(*) FROM import_jobs WHERE user_id = $1` + whereExtra
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return model.ImportJobListResult{}, err
	}

	args = append(args, perPage, offset)
	limitN := itoa(len(args) - 1)
	offsetN := itoa(len(args))

	q := `SELECT id, user_id, study_set_id, kind, status, total, imported, errors, error_msg, created_at, updated_at
		FROM import_jobs
		WHERE user_id = $1` + whereExtra + `
		ORDER BY created_at DESC
		LIMIT $` + limitN + ` OFFSET $` + offsetN

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return model.ImportJobListResult{}, err
	}
	defer rows.Close()

	var jobs []model.ImportJob
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return model.ImportJobListResult{}, err
		}
		jobs = append(jobs, j)
	}
	if err := rows.Err(); err != nil {
		return model.ImportJobListResult{}, err
	}
	if jobs == nil {
		jobs = []model.ImportJob{}
	}

	return model.ImportJobListResult{
		Items: jobs,
		PageMeta: model.PageMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: model.CalcTotalPages(total, perPage),
		},
	}, nil
}
