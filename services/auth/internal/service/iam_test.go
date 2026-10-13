package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
)

var (
	adminActor = authz.Principal{UserID: 1, LegacyRole: "admin"}
	teacher    = authz.Principal{UserID: 2, PlatformRoles: []string{authz.RoleTeacher}}
	student    = authz.Principal{UserID: 3, PlatformRoles: []string{authz.RoleStudent}}
	support    = authz.Principal{UserID: 4, PlatformRoles: []string{authz.RolePlatformSupport}}
	anonymous  = authz.Principal{}
)

func newIAMService() (*service.AuthService, *mockRepo) {
	repo := newMockRepo()
	return service.NewForTest(repo, time.Hour), repo
}

func TestIAMAdminEndpointsAreDefaultDeny(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		run  func(*service.AuthService, authz.Principal) error
	}{
		{"list roles", func(s *service.AuthService, p authz.Principal) error {
			_, err := s.ListRoles(ctx, p, "req", "127.0.0.1")
			return err
		}},
		{"list assignments", func(s *service.AuthService, p authz.Principal) error {
			_, err := s.ListAssignments(ctx, p, 0, "", 0, 0, "req", "127.0.0.1")
			return err
		}},
		{"assign role", func(s *service.AuthService, p authz.Principal) error {
			_, err := s.AssignRole(ctx, p, 9, authz.RoleTeacher, "platform", nil, "req", "127.0.0.1")
			return err
		}},
		{"revoke role", func(s *service.AuthService, p authz.Principal) error {
			return s.RevokeRole(ctx, p, 1, "req", "127.0.0.1")
		}},
		{"list users", func(s *service.AuthService, p authz.Principal) error {
			_, _, err := s.ListAdminUsers(ctx, p, "", "", 0, 0, "req", "127.0.0.1")
			return err
		}},
		{"disable user", func(s *service.AuthService, p authz.Principal) error {
			_, err := s.SetUserDisabled(ctx, p, 9, true, "req", "127.0.0.1")
			return err
		}},
		{"read audit", func(s *service.AuthService, p authz.Principal) error {
			_, err := s.ListAudit(ctx, p, 0, "", 0, 0, "req", "127.0.0.1")
			return err
		}},
	}

	for _, tt := range tests {
		for _, actor := range []struct {
			name string
			p    authz.Principal
		}{{"student", student}, {"teacher", teacher}, {"anonymous", anonymous}} {
			t.Run(tt.name+"/"+actor.name, func(t *testing.T) {
				svc, _ := newIAMService()
				err := tt.run(svc, actor.p)
				if actor.name == "anonymous" {
					if !errors.Is(err, service.ErrUnauthorized) {
						t.Fatalf("anonymous error = %v, want ErrUnauthorized", err)
					}
					return
				}
				if !errors.Is(err, service.ErrForbidden) {
					t.Fatalf("%s error = %v, want ErrForbidden", actor.name, err)
				}
			})
		}
	}
}

func TestAssignRoleRequiresPrivilegeAndIsAudited(t *testing.T) {
	ctx := context.Background()
	svc, repo := newIAMService()

	// A student cannot escalate: no assignment is created and the attempt is audited as deny.
	if _, err := svc.AssignRole(ctx, student, 9, authz.RolePlatformAdmin, "platform", nil, "req-1", "10.0.0.9"); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("student assign error = %v, want ErrForbidden", err)
	}
	if len(repo.assignments) != 0 {
		t.Fatalf("student created %d assignments, want 0 (privilege escalation)", len(repo.assignments))
	}
	if len(repo.audit) != 1 || repo.audit[0].Result != "deny" || repo.audit[0].Action != string(authz.ActionIAMAssignRole) {
		t.Fatalf("expected one deny audit entry, got %+v", repo.audit)
	}

	// An admin can grant a managed role; the write is audited as allow.
	created, err := svc.AssignRole(ctx, adminActor, 9, authz.RoleTeacher, "platform", nil, "req-2", "10.0.0.9")
	if err != nil {
		t.Fatalf("admin assign failed: %v", err)
	}
	if created.RoleKey != authz.RoleTeacher || created.ScopeType != "platform" {
		t.Fatalf("unexpected assignment: %+v", created)
	}
	last := repo.audit[len(repo.audit)-1]
	if last.Result != "allow" || last.Resource != "hq:user/9" {
		t.Fatalf("expected allow audit for hq:user/9, got %+v", last)
	}
}

func TestAssignRoleRejectsUnknownRoleAndOrgScopeWithoutID(t *testing.T) {
	ctx := context.Background()
	svc, repo := newIAMService()

	if _, err := svc.AssignRole(ctx, adminActor, 9, "SuperRoot", "platform", nil, "req", ""); err == nil {
		t.Fatal("expected validation error for unknown role")
	}
	if _, err := svc.AssignRole(ctx, adminActor, 9, authz.RoleTeacher, "org", nil, "req", ""); err == nil {
		t.Fatal("expected validation error for org scope without scopeId")
	}
	if len(repo.assignments) != 0 {
		t.Fatalf("invalid requests created %d assignments, want 0", len(repo.assignments))
	}
}

func TestRevokeRoleAuditsTarget(t *testing.T) {
	ctx := context.Background()
	svc, repo := newIAMService()

	created, err := svc.AssignRole(ctx, adminActor, 9, authz.RoleTeacher, "platform", nil, "req-1", "")
	if err != nil {
		t.Fatal(err)
	}
	repo.audit = nil

	if err := svc.RevokeRole(ctx, adminActor, created.ID, "req-2", ""); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	if len(repo.assignments) != 0 {
		t.Fatalf("assignment was not removed: %+v", repo.assignments)
	}
	if len(repo.audit) != 1 || repo.audit[0].Action != string(authz.ActionIAMRevokeRole) || repo.audit[0].Resource != "hq:user/9" {
		t.Fatalf("unexpected revoke audit: %+v", repo.audit)
	}
	// Revoking a missing assignment surfaces not-found rather than panicking.
	if err := svc.RevokeRole(ctx, adminActor, created.ID, "req-3", ""); err == nil {
		t.Fatal("expected error when revoking a missing assignment")
	}
}

func TestSetUserDisabledBlocksSelfAndAudits(t *testing.T) {
	ctx := context.Background()
	svc, repo := newIAMService()
	repo.users["target@example.com"] = mockUser{User: model.User{ID: 9, Name: "target", Email: "target@example.com", Role: "user"}}

	if _, err := svc.SetUserDisabled(ctx, adminActor, adminActor.UserID, true, "req", ""); err == nil {
		t.Fatal("admin must not disable their own account")
	}
	updated, err := svc.SetUserDisabled(ctx, adminActor, 9, true, "req", "")
	if err != nil {
		t.Fatalf("disable failed: %v", err)
	}
	if !updated.Disabled {
		t.Fatal("expected disabled=true")
	}
	if len(repo.audit) != 1 || repo.audit[0].Action != string(authz.ActionUserDisable) || repo.audit[0].Result != "allow" {
		t.Fatalf("unexpected audit: %+v", repo.audit)
	}
}

func TestPlatformSupportCanReadButNotMutateIAM(t *testing.T) {
	ctx := context.Background()
	svc, _ := newIAMService()

	if _, err := svc.ListAudit(ctx, support, 0, "", 0, 0, "req", ""); err != nil {
		t.Fatalf("support should read audit: %v", err)
	}
	if _, _, err := svc.ListAdminUsers(ctx, support, "", "", 0, 0, "req", ""); err != nil {
		t.Fatalf("support should list users: %v", err)
	}
	if _, err := svc.AssignRole(ctx, support, 9, authz.RolePlatformAdmin, "platform", nil, "req", ""); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("support assign error = %v, want ErrForbidden", err)
	}
}
