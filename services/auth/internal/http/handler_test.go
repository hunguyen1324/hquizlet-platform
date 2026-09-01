package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorEnvelopeIncludesRequestIDAndDetails(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("X-Request-ID", "request-123")

	writeError(recorder, http.StatusUnauthorized, "invalid_session", "invalid or expired session")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	var body apiError
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "INVALID_SESSION" || body.RequestID != "request-123" {
		t.Fatalf("unexpected envelope: %+v", body)
	}
	if body.Details == nil {
		t.Fatal("details must be an object, not null")
	}
}

func TestValidationErrorUsesDetailsField(t *testing.T) {
	recorder := httptest.NewRecorder()
	recorder.Header().Set("X-Request-ID", "request-456")

	writeJSON(recorder, http.StatusUnprocessableEntity, apiValidationError{
		Code: "VALIDATION_ERROR", Message: "email is required", RequestID: "request-456",
		Details: map[string]any{"field": "email"},
	})

	var body apiValidationError
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Details["field"] != "email" {
		t.Fatalf("details.field = %v, want email", body.Details["field"])
	}
}
