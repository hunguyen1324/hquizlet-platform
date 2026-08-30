package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

// WriteJSON encodes value as JSON and sends it with status code.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// ErrorResponse is the standard error envelope returned by all endpoints.
type ErrorResponse struct {
	Error string `json:"error"`
}

// WriteError sends a standard JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

// WriteServiceError maps known service/repository errors to HTTP status codes.
func WriteServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		WriteError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, service.ErrForbidden):
		WriteError(w, http.StatusForbidden, "you do not have permission to perform this action")
	default:
		WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

// PathParts strips prefix from path, trims slashes, and splits by "/".
// Returns nil if the remaining path is empty.
func PathParts(path, prefix string) []string {
	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
}
