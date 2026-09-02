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
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
	Field     string         `json:"field,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// WriteError keeps the existing handler call signature while emitting the
// canonical {code,message} envelope used by Auth and the frontend client.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{Code: errorCodeForStatus(status), Message: message, Details: map[string]any{}})
}

func WriteRequestError(w http.ResponseWriter, r *http.Request, status int, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	WriteJSON(w, status, ErrorResponse{Code: errorCodeForStatus(status), Message: message, RequestID: r.Header.Get("X-Request-ID"), Details: details})
}

func WriteValidationError(w http.ResponseWriter, status int, field, message string) {
	WriteJSON(w, status, ErrorResponse{Code: "validation_error", Message: message, Details: map[string]any{"field": field}, Field: field})
}

func WriteServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		WriteError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, service.ErrUnauthorized):
		WriteError(w, http.StatusUnauthorized, "authentication required")
	case errors.Is(err, service.ErrForbidden):
		WriteError(w, http.StatusForbidden, "you do not have permission to perform this action")
	case errors.Is(err, service.ErrValidation):
		WriteError(w, http.StatusUnprocessableEntity, "invalid request")
	case errors.Is(err, service.ErrConflict):
		WriteError(w, http.StatusConflict, "resource already exists")
	default:
		WriteError(w, http.StatusInternalServerError, "internal server error")
	}
}

func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "validation_error"
	default:
		return "internal_error"
	}
}

func PathParts(path, prefix string) []string {
	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
}
