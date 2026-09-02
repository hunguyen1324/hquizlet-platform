package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/http/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/service"
)

type FileHandler struct {
	svc *service.FileService
}

func NewFileHandler(svc *service.FileService) *FileHandler {
	return &FileHandler{svc: svc}
}

// Register wires up all /v1/files/* routes.
func (h *FileHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/files/presign", h.presign)
	mux.HandleFunc("POST /v1/files/", h.router)
	mux.HandleFunc("GET /v1/files", h.list)
}

func (h *FileHandler) router(w http.ResponseWriter, r *http.Request) {
	// Parse the path: /v1/files/{id}/confirm, /v1/files/{id}, etc.
	raw := strings.TrimPrefix(r.URL.Path, "/v1/files/")
	parts := strings.Split(raw, "/")

	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "file_not_found", "file not found")
		return
	}

	fileID := parts[0]

	if len(parts) >= 2 && parts[1] == "confirm" && r.Method == http.MethodPost {
		h.confirm(w, r, fileID)
		return
	}

	if len(parts) == 1 {
		switch r.Method {
		case http.MethodGet:
			h.get(w, r, fileID)
			return
		case http.MethodDelete:
			h.delete(w, r, fileID)
			return
		}
	}

	writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
}

func (h *FileHandler) presign(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req model.PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid JSON body")
		return
	}

	resp, err := h.svc.Presign(r.Context(), userID, req)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (h *FileHandler) confirm(w http.ResponseWriter, r *http.Request, fileID string) {
	userID := middleware.GetUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	resp, err := h.svc.Confirm(r.Context(), userID, fileID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *FileHandler) get(w http.ResponseWriter, r *http.Request, fileID string) {
	userID := middleware.GetUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	meta, err := h.svc.GetFile(r.Context(), userID, fileID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, meta)
}

func (h *FileHandler) delete(w http.ResponseWriter, r *http.Request, fileID string) {
	userID := middleware.GetUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	if err := h.svc.Delete(r.Context(), userID, fileID); err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *FileHandler) list(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == 0 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))

	resp, err := h.svc.List(r.Context(), userID, page, perPage)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Error response helpers ---

type apiError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
}

func writeError(w http.ResponseWriter, status int, code, msg string) {
	reqID := w.Header().Get("X-Request-ID")
	writeJSON(w, status, apiError{Code: code, Message: msg, RequestID: reqID, Details: map[string]any{}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("[file] writeJSON encode error: %v", err)
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "file_not_found", "file not found")
	case errors.Is(err, repository.ErrNotOwner):
		writeError(w, http.StatusForbidden, "not_owner", "you do not own this file")
	case errors.Is(err, service.ErrInvalidUploadType):
		writeError(w, http.StatusBadRequest, "invalid_upload_type", "upload type must be avatar, flashcard_image, or study_set_thumbnail")
	case errors.Is(err, service.ErrInvalidContentType):
		writeError(w, http.StatusBadRequest, "invalid_content_type", "content type not allowed for this upload type")
	case errors.Is(err, service.ErrFileTooLarge):
		writeError(w, http.StatusBadRequest, "file_too_large", "file size exceeds the maximum allowed")
	case errors.Is(err, service.ErrQuotaExceeded):
		writeError(w, http.StatusTooManyRequests, "quota_exceeded", "file quota exceeded (100 files or 500 MB)")
	case errors.Is(err, service.ErrAlreadyConfirmed):
		writeError(w, http.StatusConflict, "already_confirmed", "file already confirmed")
	case errors.Is(err, service.ErrNotYetUploaded):
		writeError(w, http.StatusConflict, "not_yet_uploaded", "file not yet uploaded to storage")
	default:
		log.Printf("[file] unexpected error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
