package repository

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// QuizQuestionRepository performs all quiz_question and quiz_question_option SQL operations.
type QuizQuestionRepository struct {
	db *sql.DB
}

// NewQuizQuestionRepository creates a new repository backed by db.
func NewQuizQuestionRepository(db *sql.DB) *QuizQuestionRepository {
	return &QuizQuestionRepository{db: db}
}

// ListByStudySet returns all quiz questions for a study set with their options, ordered by position.
func (r *QuizQuestionRepository) ListByStudySet(ctx context.Context, studySetID int64) ([]model.QuizQuestion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, study_set_id, position, question_text, question_type,
		       correct_answer, time_in_seconds, audio_url, answer_explanation,
		       paragraph_text, sub_questions, tags
		FROM quiz_question
		WHERE study_set_id = $1
		ORDER BY position ASC
	`, studySetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var questions []model.QuizQuestion
	for rows.Next() {
		var q model.QuizQuestion
		var subQ []byte
		if err := rows.Scan(&q.ID, &q.StudySetID, &q.Position, &q.QuestionText, &q.QuestionType,
			&q.CorrectAnswer, &q.TimeInSeconds, &q.AudioURL, &q.AnswerExplanation,
			&q.ParagraphText, &subQ, &q.Tags); err != nil {
			return nil, err
		}
		if subQ != nil {
			q.SubQuestions = json.RawMessage(subQ)
		}
		if q.Tags == nil {
			q.Tags = []string{}
		}
		questions = append(questions, q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load options for each question
	for i := range questions {
		opts, err := r.listOptions(ctx, questions[i].ID)
		if err != nil {
			return nil, err
		}
		questions[i].Options = opts
	}

	return questions, nil
}

// listOptions returns all options for a given question, ordered by position.
func (r *QuizQuestionRepository) listOptions(ctx context.Context, questionID int64) ([]model.QuizQuestionOption, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, question_id, text, position, is_correct
		FROM quiz_question_option
		WHERE question_id = $1
		ORDER BY position ASC
	`, questionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var opts []model.QuizQuestionOption
	for rows.Next() {
		var o model.QuizQuestionOption
		if err := rows.Scan(&o.ID, &o.QuestionID, &o.Text, &o.Position, &o.IsCorrect); err != nil {
			return nil, err
		}
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

// BulkSave replaces all quiz questions for a study set within a transaction.
// It deletes existing questions and inserts new ones.
func (r *QuizQuestionRepository) BulkSave(ctx context.Context, studySetID int64, questions []model.CreateQuizQuestionInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete existing questions (cascade deletes options)
	if _, err := tx.ExecContext(ctx, "DELETE FROM quiz_question WHERE study_set_id = $1", studySetID); err != nil {
		return err
	}

	for i, qIn := range questions {
		subQJSON, err := json.Marshal(qIn.SubQuestions)
		if err != nil {
			return err
		}
		if string(subQJSON) == "null" {
			subQJSON = nil
		}

		position := qIn.Position
		if position == 0 {
			position = i
		}

		var tags []string
		if qIn.Tags != nil {
			tags = qIn.Tags
		} else {
			tags = []string{}
		}

		var qID int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO quiz_question (study_set_id, position, question_text, question_type,
			    correct_answer, time_in_seconds, audio_url, answer_explanation,
			    paragraph_text, sub_questions, tags)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			RETURNING id
		`, studySetID, position, qIn.QuestionText, qIn.QuestionType,
			qIn.CorrectAnswer, qIn.TimeInSeconds, qIn.AudioURL, qIn.AnswerExplanation,
			qIn.ParagraphText, subQJSON, tags).Scan(&qID)
		if err != nil {
			return err
		}

		for _, opt := range qIn.Options {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO quiz_question_option (question_id, text, position, is_correct)
				VALUES ($1, $2, $3, $4)
			`, qID, opt.Text, opt.Position, opt.IsCorrect); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// DeleteByStudySet removes all quiz questions for a study set.
func (r *QuizQuestionRepository) DeleteByStudySet(ctx context.Context, studySetID int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM quiz_question WHERE study_set_id = $1", studySetID)
	return err
}
