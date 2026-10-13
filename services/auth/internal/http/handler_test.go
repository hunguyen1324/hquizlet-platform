package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
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

// auditOnlyRepo stubs the repository methods the route tests reach, so they exercise
// authorization without a database. Unused methods inherit from the embedded nil interface.
type auditOnlyRepo struct{ service.Repository }

func (auditOnlyRepo) InsertAuditLog(_ context.Context, _ int64, _, _, _, _, _ string, _ map[string]any) error {
	return nil
}

func (auditOnlyRepo) ListRoles(_ context.Context) ([]repository.IAMRole, error) {
	return []repository.IAMRole{{Key: "PlatformAdmin", Name: "PlatformAdmin", Kind: "managed"}}, nil
}

func (auditOnlyRepo) ListUsers(_ context.Context, _, _, _ string, _, _ int) ([]repository.AdminUser, int, error) {
	return nil, 0, nil
}

func (auditOnlyRepo) ListAssignments(_ context.Context, _ int64, _ string, _, _ int) ([]repository.IAMAssignment, error) {
	return nil, nil
}

func (auditOnlyRepo) ListAuditLog(_ context.Context, _ int64, _ string, _, _ int) ([]repository.AuditEntry, error) {
	return nil, nil
}

func (auditOnlyRepo) ListUsersByIDs(_ context.Context, ids []int64) ([]repository.AdminUser, error) {
	return []repository.AdminUser{{ID: ids[0], Name: "User", Email: "u@example.com", Role: "user"}}, nil
}

// TestAdminRoutesFailClosed proves every Phase 1 admin route rejects unauthenticated and
// non-admin callers at the HTTP boundary (default deny), before any data access.
func TestAdminRoutesFailClosed(t *testing.T) {
	router := NewRouter(service.NewForTest(auditOnlyRepo{}, time.Hour), nil)

	admin := map[string]string{"X-User-ID": "7", "X-User-Role": "admin", "X-User-Roles": "PlatformAdmin"}
	student := map[string]string{"X-User-ID": "9", "X-User-Role": "user", "X-User-Roles": "Student"}
	teacher := map[string]string{"X-User-ID": "8", "X-User-Role": "user", "X-User-Roles": "Teacher"}

	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		want    int
	}{
		{"roles needs auth", http.MethodGet, "/v1/admin/roles", nil, http.StatusUnauthorized},
		{"users needs auth", http.MethodGet, "/v1/admin/users", nil, http.StatusUnauthorized},
		{"assignments needs auth", http.MethodGet, "/v1/admin/assignments", nil, http.StatusUnauthorized},
		{"audit needs auth", http.MethodGet, "/v1/admin/audit", nil, http.StatusUnauthorized},
		{"student denied roles", http.MethodGet, "/v1/admin/roles", student, http.StatusForbidden},
		{"student denied users", http.MethodGet, "/v1/admin/users", student, http.StatusForbidden},
		{"teacher denied assign", http.MethodPost, "/v1/admin/assignments", teacher, http.StatusForbidden},
		{"teacher denied revoke", http.MethodDelete, "/v1/admin/assignments/5", teacher, http.StatusForbidden},
		{"student denied disable", http.MethodPost, "/v1/admin/users/3/disable", student, http.StatusForbidden},
		{"student denied audit", http.MethodGet, "/v1/admin/audit", student, http.StatusForbidden},
		{"spoofed admin without roles denied", http.MethodGet, "/v1/admin/users", map[string]string{"X-User-ID": "9", "X-User-Role": "User"}, http.StatusForbidden},
		{"legacy admin header allowed", http.MethodGet, "/v1/admin/roles", map[string]string{"X-User-ID": "1", "X-User-Role": "admin"}, http.StatusOK},
		{"platform admin allowed", http.MethodGet, "/v1/admin/users", admin, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)
			if resp.Code != tt.want {
				t.Fatalf("%s %s = %d, want %d (%s)", tt.method, tt.path, resp.Code, tt.want, resp.Body.String())
			}
		})
	}
}

func TestInternalUsersRequiresToken(t *testing.T) {
	router := NewRouter(service.NewForTest(auditOnlyRepo{}, time.Hour), nil)

	req := httptest.NewRequest(http.MethodGet, "/internal/users?ids=1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("missing token = %d, want 401", resp.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/internal/users?ids=1", nil)
	req.Header.Set("X-Internal-Token", "dev-internal-token")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("valid token = %d, want 200 (%s)", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "u@example.com") {
		t.Fatalf("directory payload missing user: %s", resp.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/internal/users?ids=1", nil)
	req.Header.Set("X-Internal-Token", "wrong")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", resp.Code)
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
