package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

// FolderRepository performs all folders SQL operations.
type FolderRepository struct {
	db *sql.DB
}

// NewFolderRepository creates a new repository backed by db.
func NewFolderRepository(db *sql.DB) *FolderRepository {
	return &FolderRepository{db: db}
}

const folderCols = `id, user_id, title, description, created_at, updated_at`

func scanFolder(s interface{ Scan(...any) error }) (model.Folder, error) {
	var f model.Folder
	err := s.Scan(&f.ID, &f.UserID, &f.Title, &f.Description, &f.CreatedAt, &f.UpdatedAt)
	return f, err
}

// List returns all folders for a user ordered by updated_at DESC.
func (r *FolderRepository) List(ctx context.Context, userID int64) ([]model.Folder, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+folderCols+`, (SELECT COUNT(*) FROM folder_to_study_sets fs WHERE fs.folder_id = folders.id)
		FROM folders
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folders := []model.Folder{}
	for rows.Next() {
		var f model.Folder
		err := rows.Scan(&f.ID, &f.UserID, &f.Title, &f.Description, &f.CreatedAt, &f.UpdatedAt, &f.StudySetCount)
		if err != nil {
			return nil, err
		}
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

// Get returns a single folder by id.
func (r *FolderRepository) Get(ctx context.Context, id int64) (model.Folder, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+folderCols+` FROM folders WHERE id = $1`, id)
	f, err := scanFolder(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Folder{}, ErrNotFound
	}
	return f, err
}

// Create inserts a new folder and returns the persisted record.
func (r *FolderRepository) Create(ctx context.Context, userID int64, in model.CreateFolderInput) (model.Folder, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO folders (user_id, title, description)
		VALUES ($1, $2, $3)
		RETURNING `+folderCols,
		userID, in.Title, in.Description)
	f, err := scanFolder(row)
	return f, err
}

// Update modifies title/description of a folder.
func (r *FolderRepository) Update(ctx context.Context, id int64, in model.UpdateFolderInput) (model.Folder, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE folders SET title = $1, description = $2, updated_at = now()
		WHERE id = $3
		RETURNING `+folderCols,
		in.Title, in.Description, id)
	f, err := scanFolder(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Folder{}, ErrNotFound
	}
	return f, err
}

// Delete removes a folder (cascades to folder_to_study_sets).
func (r *FolderRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM folders WHERE id = $1", id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// IsOwner checks whether userID owns the folder.
func (r *FolderRepository) IsOwner(ctx context.Context, id, userID int64) (bool, error) {
	var owner int64
	err := r.db.QueryRowContext(ctx, "SELECT user_id FROM folders WHERE id = $1", id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return owner == userID, nil
}

// ListStudySets returns the study sets inside a folder.
func (r *FolderRepository) ListStudySets(ctx context.Context, folderID int64) ([]model.StudySet, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, s.user_id, s.title, s.description, s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM flashcards c WHERE c.study_set_id = s.id)
		FROM study_sets s
		JOIN folder_to_study_sets fs ON fs.study_set_id = s.id
		WHERE fs.folder_id = $1
		ORDER BY fs.added_at DESC
	`, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets := []model.StudySet{}
	for rows.Next() {
		var s model.StudySet
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.Description, &s.CreatedAt, &s.UpdatedAt, &s.FlashcardCount); err != nil {
			return nil, err
		}
		sets = append(sets, s)
	}
	return sets, rows.Err()
}

// AddStudySet links a study set to a folder (idempotent via ON CONFLICT DO NOTHING).
func (r *FolderRepository) AddStudySet(ctx context.Context, folderID, studySetID int64) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO folder_to_study_sets (folder_id, study_set_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, folderID, studySetID)
	return err
}

// RemoveStudySet unlinks a study set from a folder.
func (r *FolderRepository) RemoveStudySet(ctx context.Context, folderID, studySetID int64) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM folder_to_study_sets WHERE folder_id = $1 AND study_set_id = $2
	`, folderID, studySetID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
