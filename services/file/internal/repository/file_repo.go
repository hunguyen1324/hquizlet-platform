package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/model"
)

var ErrNotFound = errors.New("file not found")
var ErrNotOwner = errors.New("not owner")

type FileRepository struct {
	db *sql.DB
}

func New(db *sql.DB) *FileRepository {
	return &FileRepository{db: db}
}

// CreateFile inserts a new file record with status 'pending'.
func (r *FileRepository) CreateFile(ctx context.Context, f model.UploadedFile) (model.UploadedFile, error) {
	var out model.UploadedFile
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO uploaded_file (id, user_id, upload_type, storage_key, filename, content_type, size_bytes, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
		RETURNING id, user_id, upload_type, filename, content_type, size_bytes, status, created_at
	`, f.ID, f.UserID, f.UploadType, f.StorageKey, f.Filename, f.ContentType, f.SizeBytes).
		Scan(&out.ID, &out.UserID, &out.UploadType, &out.Filename, &out.ContentType, &out.SizeBytes, &out.Status, &out.CreatedAt)
	return out, err
}

// GetFile returns a file by ID (only if not deleted).
func (r *FileRepository) GetFile(ctx context.Context, id string) (model.UploadedFile, error) {
	var f model.UploadedFile
	err := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, upload_type, filename, content_type, size_bytes, COALESCE(public_url,''), status, created_at, confirmed_at, deleted_at
		FROM uploaded_file
		WHERE id = $1 AND status != 'deleted'
	`, id).Scan(&f.ID, &f.UserID, &f.UploadType, &f.Filename, &f.ContentType, &f.SizeBytes, &f.PublicURL, &f.Status, &f.CreatedAt, &f.ConfirmedAt, &f.DeletedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return model.UploadedFile{}, ErrNotFound
	}
	return f, err
}

// GetFileForOwner returns a file only if it belongs to the given user.
func (r *FileRepository) GetFileForOwner(ctx context.Context, id string, userID int64) (model.UploadedFile, error) {
	f, err := r.GetFile(ctx, id)
	if err != nil {
		return model.UploadedFile{}, err
	}
	if f.UserID != userID {
		return model.UploadedFile{}, ErrNotOwner
	}
	return f, nil
}

// MarkActive sets status to 'active' and records the public URL.
func (r *FileRepository) MarkActive(ctx context.Context, id string, publicURL string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE uploaded_file
		SET status = 'active', public_url = $1, confirmed_at = now()
		WHERE id = $2 AND status = 'pending'
	`, publicURL, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Check if already active (idempotent)
		var status string
		err2 := r.db.QueryRowContext(ctx, `SELECT status FROM uploaded_file WHERE id = $1`, id).Scan(&status)
		if err2 == nil && status == "active" {
			return errors.New("already_confirmed")
		}
		return errors.New("not_yet_uploaded")
	}
	return nil
}

// SoftDelete marks a file as deleted.
func (r *FileRepository) SoftDelete(ctx context.Context, id string, userID int64) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE uploaded_file
		SET status = 'deleted', deleted_at = now()
		WHERE id = $1 AND user_id = $2 AND status != 'deleted'
	`, id, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUser returns paginated active files for a user.
func (r *FileRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.FileMetadata, int, error) {
	// Count total
	var total int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM uploaded_file WHERE user_id = $1 AND status != 'deleted'
	`, userID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, upload_type, filename, content_type, size_bytes, COALESCE(public_url,''), status, created_at, confirmed_at
		FROM uploaded_file
		WHERE user_id = $1 AND status != 'deleted'
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []model.FileMetadata
	for rows.Next() {
		var f model.FileMetadata
		if err := rows.Scan(&f.FileID, &f.UploadType, &f.Filename, &f.ContentType, &f.SizeBytes, &f.URL, &f.Status, &f.CreatedAt, &f.ConfirmedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, f)
	}
	return items, total, rows.Err()
}

// CountActiveByUser returns the count and total size of active files.
func (r *FileRepository) CountActiveByUser(ctx context.Context, userID int64) (int, int64, error) {
	var count int
	var totalSize int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(size_bytes), 0)
		FROM uploaded_file
		WHERE user_id = $1 AND status = 'active'
	`, userID).Scan(&count, &totalSize)
	return count, totalSize, err
}

// CountPendingByUser returns the count of pending files.
func (r *FileRepository) CountPendingByUser(ctx context.Context, userID int64) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(COUNT(*), 0)
		FROM uploaded_file
		WHERE user_id = $1 AND status = 'pending'
	`, userID).Scan(&count)
	return count, err
}
