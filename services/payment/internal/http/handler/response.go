package handler

import (
	"encoding/json"
	"net/http"
)

type ErrorResponse struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId,omitempty"`
	Details   map[string]any `json:"details"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, ErrorResponse{
		Code:    errorCodeForStatus(status),
		Message: message,
		Details: map[string]any{},
	})
}

func WriteRequestError(w http.ResponseWriter, r *http.Request, status int, message string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	WriteJSON(w, status, ErrorResponse{
		Code:      errorCodeForStatus(status),
		Message:   message,
		RequestID: r.Header.Get("X-Request-ID"),
		Details:   details,
	})
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
	case http.StatusPaymentRequired:
		return "payment_required"
	default:
		return "internal_error"
	}
}

func PathParts(path, prefix string) []string {
	raw := path[len(prefix):]
	if raw == "" || raw == "/" {
		return nil
	}
	// Remove leading/trailing slashes
	for len(raw) > 0 && raw[0] == '/' {
		raw = raw[1:]
	}
	for len(raw) > 0 && raw[len(raw)-1] == '/' {
		raw = raw[:len(raw)-1]
	}
	if raw == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range splitPath(raw) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitPath(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
