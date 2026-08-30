package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// FlashcardRepository performs all flashcards SQL operations.
type FlashcardRepository struct {
	db *sql.DB
}

// NewFlashcardRepository creates a new repository backed by db.
func NewFlashcardRepository(db *sql.DB) *FlashcardRepository {
	return &FlashcardRepository{db: db}
}

// ListByStudySet returns all flashcards for a study set in insertion order.
func (r *FlashcardRepository) ListByStudySet(ctx context.Context, studySetID int64) ([]model.Flashcard, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, study_set_id, term, definition, starred, created_at, updated_at
		FROM flashcards
		WHERE study_set_id = $1
		ORDER BY id ASC
	`, studySetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := []model.Flashcard{}
	for rows.Next() {
		var c model.Flashcard
		if err := rows.Scan(&c.ID, &c.StudySetID, &c.Term, &c.Definition, &c.Starred, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// Get returns a single flashcard by id.
func (r *FlashcardRepository) Get(ctx context.Context, id int64) (model.Flashcard, error) {
	var c model.Flashcard
	err := r.db.QueryRowContext(ctx, `
		SELECT id, study_set_id, term, definition, starred, created_at, updated_at
		FROM flashcards WHERE id = $1
	`, id).Scan(&c.ID, &c.StudySetID, &c.Term, &c.Definition, &c.Starred, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Flashcard{}, ErrNotFound
	}
	return c, err
}

// Create inserts a new flashcard into a study set.
func (r *FlashcardRepository) Create(ctx context.Context, studySetID int64, in model.CreateFlashcardInput) (model.Flashcard, error) {
	var c model.Flashcard
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO flashcards (study_set_id, term, definition)
		VALUES ($1, $2, $3)
		RETURNING id, study_set_id, term, definition, starred, created_at, updated_at
	`, studySetID, in.Term, in.Definition).
		Scan(&c.ID, &c.StudySetID, &c.Term, &c.Definition, &c.Starred, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

// Update modifies term/definition of an existing flashcard.
func (r *FlashcardRepository) Update(ctx context.Context, id int64, in model.UpdateFlashcardInput) (model.Flashcard, error) {
	var c model.Flashcard
	err := r.db.QueryRowContext(ctx, `
		UPDATE flashcards
		SET term = $1, definition = $2, updated_at = now()
		WHERE id = $3
		RETURNING id, study_set_id, term, definition, starred, created_at, updated_at
	`, in.Term, in.Definition, id).
		Scan(&c.ID, &c.StudySetID, &c.Term, &c.Definition, &c.Starred, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Flashcard{}, ErrNotFound
	}
	return c, err
}

// ToggleStar flips the starred flag on a flashcard.
func (r *FlashcardRepository) ToggleStar(ctx context.Context, id int64) (model.Flashcard, error) {
	var c model.Flashcard
	err := r.db.QueryRowContext(ctx, `
		UPDATE flashcards
		SET starred = NOT starred, updated_at = now()
		WHERE id = $1
		RETURNING id, study_set_id, term, definition, starred, created_at, updated_at
	`, id).Scan(&c.ID, &c.StudySetID, &c.Term, &c.Definition, &c.Starred, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Flashcard{}, ErrNotFound
	}
	return c, err
}

// Delete removes a flashcard by id.
func (r *FlashcardRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM flashcards WHERE id = $1", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
