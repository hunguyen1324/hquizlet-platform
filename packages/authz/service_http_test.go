package authz

import (
	"net/http/httptest"
	"testing"
)

func TestServiceRequestCannotChangeProgressScope(t *testing.T) {
	r := httptest.NewRequest("GET", "/internal/progress/summary?userIds=1&studySetIds=2", nil)
	SignServiceRequest(r, "test-secret")
	if !VerifyServiceRequest(r, "test-secret") {
		t.Fatal("valid signature rejected")
	}
	r.URL.RawQuery = "userIds=3&studySetIds=2"
	if VerifyServiceRequest(r, "test-secret") {
		t.Fatal("changed user scope accepted")
	}
	if VerifyServiceRequest(r, "") {
		t.Fatal("missing signing key accepted")
	}
	r.Header.Set("X-Service-Timestamp", "0")
	if VerifyServiceRequest(r, "test-secret") {
		t.Fatal("expired proof accepted")
	}
}
