package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteErrorUsesCanonicalEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteError(recorder, http.StatusUnauthorized, "authentication required")

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}

	var body ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "unauthorized" {
		t.Fatalf("code = %q, want %q", body.Code, "unauthorized")
	}
	if body.Message != "authentication required" {
		t.Fatalf("message = %q, want %q", body.Message, "authentication required")
	}
}

func TestWriteValidationErrorIncludesField(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteValidationError(recorder, http.StatusUnprocessableEntity, "title", "title is required")

	var body ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != "validation_error" || body.Field != "title" || body.Message != "title is required" {
		t.Fatalf("unexpected validation response: %+v", body)
	}
}
