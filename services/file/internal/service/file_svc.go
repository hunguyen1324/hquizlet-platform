package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/config"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/storage"
)

var (
	ErrInvalidUploadType  = fmt.Errorf("invalid_upload_type")
	ErrInvalidContentType = fmt.Errorf("invalid_content_type")
	ErrFileTooLarge       = fmt.Errorf("file_too_large")
	ErrQuotaExceeded      = fmt.Errorf("quota_exceeded")
	ErrAlreadyConfirmed   = fmt.Errorf("already_confirmed")
	ErrNotYetUploaded     = fmt.Errorf("not_yet_uploaded")
)

var (
	validUploadTypes = map[string]bool{
		model.UploadTypeAvatar:             true,
		model.UploadTypeFlashcardImage:     true,
		model.UploadTypeStudySetThumbnail:  true,
	}
	sanitizeRE = regexp.MustCompile(`[^a-z0-9._-]`)
)

type FileService struct {
	repo    *repository.FileRepository
	storage storage.Storage
	cfg     config.Config
}

func New(repo *repository.FileRepository, storage storage.Storage, cfg config.Config) *FileService {
	return &FileService{repo: repo, storage: storage, cfg: cfg}
}

// Presign validates input, creates a pending file record, and returns a presigned URL.
func (s *FileService) Presign(ctx context.Context, userID int64, req model.PresignRequest) (model.PresignResponse, error) {
	// Validate upload type
	if !validUploadTypes[req.UploadType] {
		return model.PresignResponse{}, ErrInvalidUploadType
	}

	// Validate MIME type
	if !s.isAllowedMIME(req.UploadType, req.ContentType) {
		return model.PresignResponse{}, ErrInvalidContentType
	}

	// Validate file size
	maxSize := model.MaxSizeForType(req.UploadType)
	if req.FileSize <= 0 || req.FileSize > maxSize {
		return model.PresignResponse{}, ErrFileTooLarge
	}

	// Check quota (active + pending counts toward limit)
	activeCount, activeBytes, err := s.repo.CountActiveByUser(ctx, userID)
	if err != nil {
		return model.PresignResponse{}, fmt.Errorf("quota check: %w", err)
	}
	pendingCount, err := s.repo.CountPendingByUser(ctx, userID)
	if err != nil {
		return model.PresignResponse{}, fmt.Errorf("pending check: %w", err)
	}

	totalCount := activeCount + pendingCount
	if totalCount >= model.MaxFilesPerUser {
		return model.PresignResponse{}, ErrQuotaExceeded
	}
	if activeBytes+req.FileSize > model.MaxBytesPerUser {
		return model.PresignResponse{}, ErrQuotaExceeded
	}

	// Generate file ID (UUID)
	fileID := generateUUID()

	// Build storage key
	sanitized := s.sanitizeFilename(req.Filename)
	storageKey := s.buildStorageKey(userID, req.UploadType, fileID, sanitized)

	// Create pending record
	record := model.UploadedFile{
		ID:          fileID,
		UserID:      userID,
		UploadType:  req.UploadType,
		StorageKey:  storageKey,
		Filename:    req.Filename,
		ContentType: req.ContentType,
		SizeBytes:   req.FileSize,
		Status:      model.StatusPending,
	}
	if _, err := s.repo.CreateFile(ctx, record); err != nil {
		return model.PresignResponse{}, fmt.Errorf("create file record: %w", err)
	}

	// Generate presigned URL
	ttl := s.cfg.PresignTTL()
	uploadURL, expiresAt, err := s.storage.PresignPut(ctx, storageKey, req.ContentType, ttl)
	if err != nil {
		return model.PresignResponse{}, fmt.Errorf("presign: %w", err)
	}

	return model.PresignResponse{
		FileID:       fileID,
		UploadURL:    uploadURL,
		UploadMethod: "PUT",
		ExpiresAt:    expiresAt,
		HeadersRequired: map[string]string{
			"Content-Type": req.ContentType,
		},
	}, nil
}

// Confirm verifies the upload exists and activates the file record.
func (s *FileService) Confirm(ctx context.Context, userID int64, fileID string) (model.ConfirmResponse, error) {
	// Get file and check ownership
	f, err := s.repo.GetFileForOwner(ctx, fileID, userID)
	if err != nil {
		return model.ConfirmResponse{}, err
	}

	if f.Status == model.StatusActive {
		return model.ConfirmResponse{}, ErrAlreadyConfirmed
	}

	// Verify object exists in storage
	_, _, err = s.storage.HeadObject(ctx, f.StorageKey)
	if err != nil {
		return model.ConfirmResponse{}, ErrNotYetUploaded
	}

	// Generate public URL
	publicURL, err := s.storage.PublicURL(ctx, f.StorageKey, 0)
	if err != nil {
		return model.ConfirmResponse{}, fmt.Errorf("generate public URL: %w", err)
	}

	// Mark active
	if err := s.repo.MarkActive(ctx, fileID, publicURL); err != nil {
		return model.ConfirmResponse{}, err
	}

	return model.ConfirmResponse{
		FileID:      fileID,
		URL:         publicURL,
		UploadType:  f.UploadType,
		SizeBytes:   f.SizeBytes,
		ConfirmedAt: time.Now().UTC(),
	}, nil
}

// GetFile returns metadata for a file (owner only).
func (s *FileService) GetFile(ctx context.Context, userID int64, fileID string) (model.FileMetadata, error) {
	f, err := s.repo.GetFileForOwner(ctx, fileID, userID)
	if err != nil {
		return model.FileMetadata{}, err
	}
	return model.FileMetadata{
		FileID:      f.ID,
		UploadType:  f.UploadType,
		Filename:    f.Filename,
		ContentType: f.ContentType,
		SizeBytes:   f.SizeBytes,
		URL:         f.PublicURL,
		Status:      f.Status,
		CreatedAt:   f.CreatedAt,
		ConfirmedAt: f.ConfirmedAt,
	}, nil
}

// Delete soft-deletes a file (owner only).
func (s *FileService) Delete(ctx context.Context, userID int64, fileID string) error {
	return s.repo.SoftDelete(ctx, fileID, userID)
}

// List returns paginated files and quota for the user.
func (s *FileService) List(ctx context.Context, userID int64, page, perPage int) (model.FileListResponse, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	items, total, err := s.repo.ListByUser(ctx, userID, perPage, offset)
	if err != nil {
		return model.FileListResponse{}, err
	}

	activeCount, usedBytes, err := s.repo.CountActiveByUser(ctx, userID)
	if err != nil {
		return model.FileListResponse{}, err
	}

	return model.FileListResponse{
		Items: items,
		Total: total,
		Quota: model.QuotaInfo{
			ActiveFiles: activeCount,
			MaxFiles:    model.MaxFilesPerUser,
			UsedBytes:   usedBytes,
			MaxBytes:    model.MaxBytesPerUser,
		},
	}, nil
}

// --- helpers ---

func (s *FileService) isAllowedMIME(uploadType, contentType string) bool {
	allowed, ok := model.AllowedMIMETypes[uploadType]
	if !ok {
		return false
	}
	for _, m := range allowed {
		if strings.EqualFold(m, contentType) {
			return true
		}
	}
	return false
}

func (s *FileService) sanitizeFilename(name string) string {
	// Lowercase
	name = strings.ToLower(name)
	// Take only the basename (no path)
	name = path.Base(name)
	// Replace unsafe characters
	name = sanitizeRE.ReplaceAllString(name, "_")
	// Trim to 100 chars
	if len(name) > 100 {
		name = name[:100]
	}
	// Ensure non-empty
	if name == "" || name == "." || name == "_" {
		name = "file"
	}
	return name
}

func (s *FileService) buildStorageKey(userID int64, uploadType, fileID, filename string) string {
	return fmt.Sprintf("uploads/%d/%s/%s/%s", userID, uploadType, fileID, filename)
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// URLValidation validates a URL string (for image_url / thumbnail_url fields).
func URLValidation(raw string) error {
	if raw == "" {
		return nil // null/empty is allowed (clears the URL)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL format")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme")
	}
	if len(raw) > 2000 {
		return fmt.Errorf("URL must not exceed 2000 characters")
	}
	return nil
}
