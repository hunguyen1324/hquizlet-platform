package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

// ClassStudySetRepository implements model.ClassStudySetRepository backed by PostgreSQL.
type ClassStudySetRepository struct {
	db *sql.DB
}

func NewClassStudySetRepository(db *sql.DB) *ClassStudySetRepository {
	return &ClassStudySetRepository{db: db}
}

// Add inserts a study set assignment. Returns ErrConflict if already assigned.
func (r *ClassStudySetRepository) Add(ctx context.Context, classID, studySetID, addedByUserID int64) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO class_study_sets (class_id, study_set_id, added_by_user_id)
		VALUES ($1, $2, $3)
	`, classID, studySetID, addedByUserID)
	if err != nil {
		if isUniqueViolation(err) {
			return ErrConflict
		}
		return err
	}
	return nil
}

// List returns all study sets assigned to a class.
func (r *ClassStudySetRepository) List(ctx context.Context, classID int64) ([]*model.ClassStudySet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT css.class_id, css.study_set_id, css.added_by_user_id, css.added_at
		FROM class_study_sets css
		WHERE css.class_id = $1
		ORDER BY css.added_at DESC
	`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sets []*model.ClassStudySet
	for rows.Next() {
		s := &model.ClassStudySet{}
		if err := rows.Scan(&s.ClassID, &s.StudySetID, &s.AddedByUserID, &s.AddedAt); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

// Remove deletes a study set assignment from a class.
func (r *ClassStudySetRepository) Remove(ctx context.Context, classID, studySetID int64) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM class_study_sets WHERE class_id = $1 AND study_set_id = $2
	`, classID, studySetID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountByClass returns the number of study sets assigned to a class.
func (r *ClassStudySetRepository) CountByClass(ctx context.Context, classID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM class_study_sets WHERE class_id = $1
	`, classID).Scan(&count)
	return count, err
}

// hasConflict checks if an error is a unique constraint violation.
func hasConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}
