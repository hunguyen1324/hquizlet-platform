package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, ErrorResponse{Code: code, Message: message})
}

func WriteValidationError(w http.ResponseWriter, status int, field, message string) {
	WriteJSON(w, status, ErrorResponse{Code: "validation_error", Message: message, Field: field})
}

func WriteServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		WriteError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, service.ErrUnauthorized):
		WriteError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
	case errors.Is(err, service.ErrForbidden):
		WriteError(w, http.StatusForbidden, "forbidden", "you do not have permission to perform this action")
	default:
		WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func PathParts(path, prefix string) []string {
	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
}
