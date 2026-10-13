package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
)

var _ service.Repository = (*mockRepo)(nil)

func TestDisabledAccountCannotCreateLoginSession(t *testing.T) {
	svc, repo := newIAMService()
	ctx := context.Background()
	in := model.RegisterInput{Name: "Student", Email: "disabled@example.com", Password: "Password123!", AccountType: "student"}
	registered, err := svc.Register(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	actor := authz.Principal{UserID: 99, PlatformRoles: []string{authz.RolePlatformAdmin}}
	if _, err := svc.SetUserDisabled(ctx, actor, registered.User.ID, true, "req", ""); err != nil {
		t.Fatal(err)
	}
	before := len(repo.sessions)
	_, err = svc.Login(ctx, model.LoginInput{Email: in.Email, Password: in.Password})
	if !errors.Is(err, service.ErrDisabledUser) {
		t.Fatalf("want disabled error, got %v", err)
	}
	if len(repo.sessions) != before {
		t.Fatal("disabled login created a session")
	}
	if _, err := svc.Me(ctx, registered.Token); err == nil {
		t.Fatal("old session still accepted")
	}
}

func (m *mockRepo) ListOrgRoleEntries(_ context.Context, userID int64) ([]string, error) {
	var out []string
	for _, a := range m.assignments {
		if a.PrincipalType == "user" && a.PrincipalID == userID && a.ScopeType == "org" && a.ScopeID != nil && (a.ExpiresAt == nil || a.ExpiresAt.After(time.Now())) {
			out = append(out, authz.FormatOrgRole(*a.ScopeID, a.RoleKey))
		}
	}
	return out, nil
}

func (m *mockRepo) OrganizationExists(_ context.Context, id int64) (bool, error) {
	for _, o := range m.organizations {
		if o.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockRepo) ListOrganizations(_ context.Context, limit, offset int) ([]repository.Organization, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	out := make([]repository.Organization, 0)
	for i := len(m.organizations) - 1 - offset; i >= 0 && len(out) < limit; i-- {
		out = append(out, m.organizations[i])
	}
	return out, len(m.organizations), nil
}

func (m *mockRepo) CreateOrganization(_ context.Context, name, slug, kind string, owner int64) (repository.Organization, error) {
	for _, o := range m.organizations {
		if o.Slug == slug {
			return repository.Organization{}, repository.ErrConflict
		}
	}
	o := repository.Organization{ID: int64(len(m.organizations) + 1), Name: name, Slug: slug, Kind: kind, OwnerUserID: owner, CreatedAt: time.Now()}
	m.organizations = append(m.organizations, o)
	return o, nil
}

func TestOrganizationAssignmentsValidateRoleAndScope(t *testing.T) {
	orgID, missingID := int64(1), int64(999)
	for _, tt := range []struct {
		name, role, scope string
		id                *int64
		valid             bool
	}{
		{"org admin on platform", authz.RoleOrgAdmin, "platform", nil, false},
		{"platform admin in org", authz.RolePlatformAdmin, "org", &orgID, false},
		{"teacher in org", authz.RoleTeacher, "org", &orgID, false},
		{"missing org", authz.RoleOrgTeacher, "org", &missingID, false},
		{"missing scope ID", authz.RoleOrgStudent, "org", nil, false},
		{"valid org admin", authz.RoleOrgAdmin, "org", &orgID, true},
		{"valid org teacher", authz.RoleOrgTeacher, "org", &orgID, true},
		{"valid org student", authz.RoleOrgStudent, "org", &orgID, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo := newIAMService()
			repo.organizations = []repository.Organization{{ID: orgID}}
			got, err := svc.AssignRole(context.Background(), adminActor, 9, tt.role, tt.scope, tt.id, "req", "")
			if tt.valid {
				if err != nil {
					t.Fatal(err)
				}
				if got.ScopeID == nil || *got.ScopeID != orgID {
					t.Fatalf("unexpected scope: %+v", got)
				}
				if len(repo.assignments) != 1 || len(repo.audit) != 1 || repo.audit[0].Result != "allow" {
					t.Fatal("expected assignment and allow audit")
				}
			} else {
				var validation *service.ValidationError
				if !errors.As(err, &validation) {
					t.Fatalf("want validation error, got %v", err)
				}
				if len(repo.assignments) != 0 {
					t.Fatal("invalid role/scope persisted")
				}
			}
		})
	}
}

func TestOrganizationAdminCannotEscalatePlatformPrivileges(t *testing.T) {
	svc, repo := newIAMService()
	actor := authz.Principal{UserID: 9, OrgRoles: map[int64][]string{1: {authz.RoleOrgAdmin}}}
	_, err := svc.AssignRole(context.Background(), actor, 9, authz.RolePlatformAdmin, "platform", nil, "req", "")
	if !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
	if len(repo.assignments) != 0 || len(repo.audit) != 1 || repo.audit[0].Result != "deny" {
		t.Fatal("escalation must be rejected and audited")
	}
}
