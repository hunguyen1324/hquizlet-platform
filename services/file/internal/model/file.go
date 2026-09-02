package model

import (
	"time"
)

// UploadedFile represents a file metadata record in PostgreSQL.
type UploadedFile struct {
	ID          string     `json:"file_id"`
	UserID      int64      `json:"-"`
	UserIDStr   string     `json:"userId,omitempty"`
	UploadType  string     `json:"upload_type"`
	StorageKey  string     `json:"-"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
	PublicURL   string     `json:"url,omitempty"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

// PresignRequest is the input for POST /v1/files/presign.
type PresignRequest struct {
	UploadType  string `json:"upload_type"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	FileSize    int64  `json:"file_size"`
}

// PresignResponse is the output for POST /v1/files/presign.
type PresignResponse struct {
	FileID          string            `json:"file_id"`
	UploadURL       string            `json:"upload_url"`
	UploadMethod    string            `json:"upload_method"`
	ExpiresAt       time.Time         `json:"expires_at"`
	HeadersRequired map[string]string `json:"headers_required"`
}

// ConfirmResponse is the output for POST /v1/files/{id}/confirm.
type ConfirmResponse struct {
	FileID      string    `json:"file_id"`
	URL         string    `json:"url"`
	UploadType  string    `json:"upload_type"`
	SizeBytes   int64     `json:"size_bytes"`
	ConfirmedAt time.Time `json:"confirmed_at"`
}

// FileMetadata is a single file in the list response.
type FileMetadata struct {
	FileID      string     `json:"file_id"`
	UploadType  string     `json:"upload_type"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
	URL         string     `json:"url"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
}

// QuotaInfo represents the user's storage quota.
type QuotaInfo struct {
	ActiveFiles int   `json:"active_files"`
	MaxFiles    int   `json:"max_files"`
	UsedBytes   int64 `json:"used_bytes"`
	MaxBytes    int64 `json:"max_bytes"`
}

// FileListResponse is the output for GET /v1/files.
type FileListResponse struct {
	Items []FileMetadata `json:"items"`
	Total int            `json:"total"`
	Quota QuotaInfo      `json:"quota"`
}

// Upload type constants
const (
	UploadTypeAvatar           = "avatar"
	UploadTypeFlashcardImage   = "flashcard_image"
	UploadTypeStudySetThumbnail = "study_set_thumbnail"
)

// Status constants
const (
	StatusPending = "pending"
	StatusActive  = "active"
	StatusDeleted = "deleted"
)

// Max file sizes in bytes
const (
	MaxAvatarSize          = 5 * 1024 * 1024   // 5 MB
	MaxFlashcardImageSize  = 10 * 1024 * 1024   // 10 MB
	MaxStudySetThumbSize   = 5 * 1024 * 1024    // 5 MB
	MaxFilesPerUser        = 100
	MaxBytesPerUser        = 500 * 1024 * 1024  // 500 MB
)

// MIME type whitelists per upload type
var AllowedMIMETypes = map[string][]string{
	UploadTypeAvatar:             {"image/jpeg", "image/png", "image/webp"},
	UploadTypeFlashcardImage:     {"image/jpeg", "image/png", "image/webp", "image/gif"},
	UploadTypeStudySetThumbnail:  {"image/jpeg", "image/png", "image/webp"},
}

// ValidUploadTypes is the set of valid upload type values.
var ValidUploadTypes = map[string]bool{
	UploadTypeAvatar:             true,
	UploadTypeFlashcardImage:     true,
	UploadTypeStudySetThumbnail:  true,
}

// MaxSizeForType returns the max file size for the given upload type.
func MaxSizeForType(uploadType string) int64 {
	switch uploadType {
	case UploadTypeAvatar:
		return MaxAvatarSize
	case UploadTypeFlashcardImage:
		return MaxFlashcardImageSize
	case UploadTypeStudySetThumbnail:
		return MaxStudySetThumbSize
	default:
		return 0
	}
}
