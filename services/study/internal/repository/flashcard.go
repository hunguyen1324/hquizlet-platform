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

// scanCard scans a full flashcard row (includes position).
func scanCard(s interface {
	Scan(...any) error
}) (model.Flashcard, error) {
	var c model.Flashcard
	err := s.Scan(&c.ID, &c.StudySetID, &c.Term, &c.Definition, &c.ExampleSentence, &c.HintExplanation, &c.Synonyms, &c.ImageURL, &c.Starred, &c.Position, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

const selectCols = `id, study_set_id, term, definition, example_sentence, hint_explanation, synonyms, image_url, starred, position, created_at, updated_at`

// ListByStudySet returns all flashcards for a study set ordered by position then id.
func (r *FlashcardRepository) ListByStudySet(ctx context.Context, studySetID int64) ([]model.Flashcard, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+selectCols+`
		FROM flashcards
		WHERE study_set_id = $1
		ORDER BY position ASC, id ASC
	`, studySetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := []model.Flashcard{}
	for rows.Next() {
		c, err := scanCard(rows)
		if err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

// Get returns a single flashcard by id.
func (r *FlashcardRepository) Get(ctx context.Context, id int64) (model.Flashcard, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+selectCols+` FROM flashcards WHERE id = $1`, id)
	c, err := scanCard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Flashcard{}, ErrNotFound
	}
	return c, err
}

// Create inserts a new flashcard into a study set.
func (r *FlashcardRepository) Create(ctx context.Context, studySetID int64, in model.CreateFlashcardInput) (model.Flashcard, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO flashcards (study_set_id, term, definition, example_sentence, hint_explanation, synonyms, image_url, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE((SELECT MAX(position)+1 FROM flashcards WHERE study_set_id = $1), 0))
		RETURNING `+selectCols,
		studySetID, in.Term, in.Definition, in.ExampleSentence, in.HintExplanation, in.Synonyms, in.ImageURL)
	c, err := scanCard(row)
	return c, err
}

// Update modifies term/definition/position/image_url of an existing flashcard.
func (r *FlashcardRepository) Update(ctx context.Context, id int64, in model.UpdateFlashcardInput) (model.Flashcard, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE flashcards
		SET term = $1, definition = $2, example_sentence = $3, hint_explanation = $4, synonyms = $5, image_url = $6, updated_at = now()
		WHERE id = $7
		RETURNING `+selectCols,
		in.Term, in.Definition, in.ExampleSentence, in.HintExplanation, in.Synonyms, in.ImageURL, id)
	c, err := scanCard(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Flashcard{}, ErrNotFound
	}
	return c, err
}

// ToggleStar flips the starred flag on a flashcard.
func (r *FlashcardRepository) ToggleStar(ctx context.Context, id int64) (model.Flashcard, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE flashcards
		SET starred = NOT starred, updated_at = now()
		WHERE id = $1
		RETURNING `+selectCols, id)
	c, err := scanCard(row)
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

// BulkSave performs create/update/delete in a single transaction.
// Only cards belonging to studySetID are modified; cards with wrong ownership are skipped.
func (r *FlashcardRepository) BulkSave(ctx context.Context, studySetID int64, items []model.BulkFlashcardItem) (model.BulkSaveResult, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return model.BulkSaveResult{}, err
	}
	defer tx.Rollback() //nolint:errcheck

	var result model.BulkSaveResult
	result.Created = []model.Flashcard{}
	result.Updated = []model.Flashcard{}
	result.Deleted = []int64{}

	for _, item := range items {
		if item.Delete && item.ID > 0 {
			// Verify ownership before delete
			var ownerSetID int64
			err := tx.QueryRowContext(ctx, "SELECT study_set_id FROM flashcards WHERE id = $1", item.ID).Scan(&ownerSetID)
			if err != nil || ownerSetID != studySetID {
				continue // skip cards not belonging to this set
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM flashcards WHERE id = $1", item.ID); err != nil {
				return model.BulkSaveResult{}, err
			}
			result.Deleted = append(result.Deleted, item.ID)
			continue
		}

		if item.ID == 0 {
			// Create new card
			row := tx.QueryRowContext(ctx, `
				INSERT INTO flashcards (study_set_id, term, definition, example_sentence, hint_explanation, synonyms, image_url, position)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				RETURNING `+selectCols,
				studySetID, item.Term, item.Definition, item.ExampleSentence, item.HintExplanation, item.Synonyms, item.ImageURL, item.Position)
			c, err := scanCard(row)
			if err != nil {
				return model.BulkSaveResult{}, err
			}
			result.Created = append(result.Created, c)
		} else {
			// Update existing card – check ownership
			var ownerSetID int64
			err := tx.QueryRowContext(ctx, "SELECT study_set_id FROM flashcards WHERE id = $1", item.ID).Scan(&ownerSetID)
			if err != nil || ownerSetID != studySetID {
				continue
			}
			row := tx.QueryRowContext(ctx, `
				UPDATE flashcards SET term = $1, definition = $2, example_sentence = $3, hint_explanation = $4, synonyms = $5, image_url = $6, position = $7, updated_at = now()
				WHERE id = $8
				RETURNING `+selectCols,
				item.Term, item.Definition, item.ExampleSentence, item.HintExplanation, item.Synonyms, item.ImageURL, item.Position, item.ID)
			c, err := scanCard(row)
			if err != nil {
				return model.BulkSaveResult{}, err
			}
			result.Updated = append(result.Updated, c)
		}
	}

	if err := tx.Commit(); err != nil {
		return model.BulkSaveResult{}, err
	}
	return result, nil
}
