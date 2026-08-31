package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

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

// ListAll is intentionally not exposed through the StudySets interface.
// Study API resources are always user-scoped after authentication.
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

// ListWithFilter returns paginated study sets for a user with optional search and sort.
func (r *StudySetRepository) ListWithFilter(ctx context.Context, userID int64, f model.StudySetFilter) (model.StudySetListResult, error) {
	perPage := f.PerPage
	if perPage <= 0 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}
	page := f.Page
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * perPage

	orderCol := "updated_at"
	switch f.SortBy {
	case "created":
		orderCol = "created_at"
	case "title":
		orderCol = "title"
	}

	args := []any{userID}
	whereExtra := ""
	if f.Search != "" {
		args = append(args, "%"+f.Search+"%")
		whereExtra = " AND title ILIKE $" + itoa(len(args))
	}

	var total int
	countQ := "SELECT COUNT(*) FROM study_sets WHERE user_id = $1" + whereExtra
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return model.StudySetListResult{}, err
	}

	args = append(args, perPage, offset)
	limitN := itoa(len(args) - 1)
	offsetN := itoa(len(args))
	q := `SELECT id, user_id, title, description, created_at, updated_at
		FROM study_sets
		WHERE user_id = $1` + whereExtra +
		` ORDER BY ` + orderCol + ` DESC, id DESC
		LIMIT $` + limitN + ` OFFSET $` + offsetN

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return model.StudySetListResult{}, err
	}
	defer rows.Close()

	sets := []model.StudySet{}
	for rows.Next() {
		var s model.StudySet
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return model.StudySetListResult{}, err
		}
		sets = append(sets, s)
	}
	if err := rows.Err(); err != nil {
		return model.StudySetListResult{}, err
	}

	totalPages := total / perPage
	if total%perPage != 0 {
		totalPages++
	}

	return model.StudySetListResult{
		Items:      sets,
		Total:      total,
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
	}, nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
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

// GetOwned returns a study set only when it belongs to userID.
// Ownership is enforced in SQL so callers cannot accidentally fetch another user's set.
func (r *StudySetRepository) GetOwned(ctx context.Context, id, userID int64) (model.StudySet, error) {
	var s model.StudySet
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, description, created_at, updated_at
		FROM study_sets
		WHERE id = $1 AND user_id = $2
	`, id, userID).Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.StudySet{}, ErrForbidden
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

// ErrForbidden is returned when a resource exists but is owned by another user.
var ErrForbidden = errors.New("forbidden")
