package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
)

type expiredGrantRepo struct{ *mockRepo }

func (*expiredGrantRepo) ListPlatformRoleKeys(context.Context, int64) ([]string, error) {
	return []string{}, nil
}
func (*expiredGrantRepo) HasPlatformAssignment(context.Context, int64) (bool, error) {
	return true, nil
}

func TestExpiredManagedGrantDoesNotReviveLegacyAdmin(t *testing.T) {
	repo := &expiredGrantRepo{newMockRepo()}
	svc := service.NewForTest(repo, time.Hour)
	session, err := svc.Register(context.Background(), model.RegisterInput{Name: "Legacy", Email: "legacy@test.local", Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	u := repo.users["legacy@test.local"]
	u.Role = "admin"
	repo.users["legacy@test.local"] = u
	identity, err := svc.VerifyToken(context.Background(), session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Role == "admin" || len(identity.PlatformRoles) != 0 {
		t.Fatalf("expired grant revived: %+v", identity)
	}
	permissions, err := svc.Permissions(context.Background(), session.Token, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range permissions.Actions {
		if action == "dashboard:ViewAdmin" || action == "wallet:Credit" {
			t.Fatalf("expired admin still allowed %s", action)
		}
	}
}
