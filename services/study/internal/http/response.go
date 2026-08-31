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
	Error string `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Error: message})
}

func WriteServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		WriteError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, service.ErrUnauthorized):
		WriteError(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, service.ErrForbidden):
		WriteError(w, http.StatusForbidden, "you do not have permission to perform this action")
	default:
		WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func PathParts(path, prefix string) []string {
	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if raw == "" { return nil }
	return strings.Split(raw, "/")
}
