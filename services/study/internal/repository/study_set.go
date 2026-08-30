package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("resource not found")

// StudySetRepository performs all study_sets SQL operations.
type StudySetRepository struct {
	db *sql.DB
}

// NewStudySetRepository creates a new repository backed by db.
func NewStudySetRepository(db *sql.DB) *StudySetRepository {
	return &StudySetRepository{db: db}
}

// List returns all study sets for a user ordered by last updated.
func (r *StudySetRepository) List(ctx context.Context, userID int64) ([]model.StudySet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, title, description, created_at, updated_at
		FROM study_sets
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets := []model.StudySet{}
	for rows.Next() {
		var s model.StudySet
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

// ListAll returns every study set (used when no auth is present, e.g. public sets).
func (r *StudySetRepository) ListAll(ctx context.Context) ([]model.StudySet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, title, description, created_at, updated_at
		FROM study_sets
		ORDER BY updated_at DESC, id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets := []model.StudySet{}
	for rows.Next() {
		var s model.StudySet
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

// Get returns a single study set by id, without flashcards.
func (r *StudySetRepository) Get(ctx context.Context, id int64) (model.StudySet, error) {
	var s model.StudySet
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, description, created_at, updated_at
		FROM study_sets WHERE id = $1
	`, id).Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.StudySet{}, ErrNotFound
	}
	return s, err
}

// Create inserts a new study set and returns the persisted record.
func (r *StudySetRepository) Create(ctx context.Context, userID int64, in model.CreateStudySetInput) (model.StudySet, error) {
	var s model.StudySet
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO study_sets (user_id, title, description)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, title, description, created_at, updated_at
	`, userID, in.Title, in.Description).
		Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.CreatedAt, &s.UpdatedAt)
	return s, err
}

// Update modifies an existing study set and returns the updated record.
func (r *StudySetRepository) Update(ctx context.Context, id int64, in model.UpdateStudySetInput) (model.StudySet, error) {
	var s model.StudySet
	err := r.db.QueryRowContext(ctx, `
		UPDATE study_sets
		SET title = $1, description = $2, updated_at = now()
		WHERE id = $3
		RETURNING id, user_id, title, description, created_at, updated_at
	`, in.Title, in.Description, id).
		Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.StudySet{}, ErrNotFound
	}
	return s, err
}

// Delete removes a study set (cascades to flashcards via FK).
func (r *StudySetRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM study_sets WHERE id = $1", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// IsOwner checks whether userID is the owner of the study set.
func (r *StudySetRepository) IsOwner(ctx context.Context, id, userID int64) (bool, error) {
	var owner int64
	err := r.db.QueryRowContext(ctx, "SELECT user_id FROM study_sets WHERE id = $1", id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return owner == userID, nil
}
