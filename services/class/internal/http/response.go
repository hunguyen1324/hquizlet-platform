package http

import (
	"encoding/json"
	"net/http"
	"strings"
)

// ErrorResponse is the standard error envelope.
type ErrorResponse struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// WriteError writes an error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{
		Code:    errorCodeForStatus(status),
		Message: message,
		Details: map[string]any{},
	})
}

// WriteErrorWithRequestID writes an error response with request ID.
func WriteErrorWithRequestID(w http.ResponseWriter, r *http.Request, status int, message string) {
	WriteJSON(w, status, ErrorResponse{
		Code:      errorCodeForStatus(status),
		Message:   message,
		RequestID: r.Header.Get("X-Request-ID"),
		Details:   map[string]any{},
	})
}

// PathParts extracts path segments after a prefix.
func PathParts(path, prefix string) []string {
	raw := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
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
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "validation_error"
	case http.StatusTooManyRequests:
		return "rate_limit_exceeded"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "internal_error"
	}
}
