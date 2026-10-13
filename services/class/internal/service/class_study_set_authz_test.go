package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

func TestClassAssignmentsUseScopedPolicy(t *testing.T) {
	svc := NewClassStudySetService(&fakeClassStore{class: &model.Class{ID: 10, OwnerUserID: 1}}, &fakeMemberStore{roles: map[int64]string{1: "owner", 2: "teacher", 3: "student"}}, &fakeStudySetStore{}, nil)
	for _, test := range []struct {
		name    string
		actor   authz.Principal
		allowed bool
	}{
		{"owner", authz.Principal{UserID: 1}, true},
		{"class teacher", authz.Principal{UserID: 2}, true},
		{"student", authz.Principal{UserID: 3, PlatformRoles: []string{authz.RoleStudent}}, false},
		{"outsider teacher", authz.Principal{UserID: 99, PlatformRoles: []string{authz.RoleTeacher}}, false},
		{"outsider admin", authz.Principal{UserID: 99, PlatformRoles: []string{authz.RolePlatformAdmin}}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range []error{svc.AddStudySet(context.Background(), test.actor, 10, 100), svc.RemoveStudySet(context.Background(), test.actor, 10, 100)} {
				if test.allowed && err != nil {
					t.Fatal(err)
				}
				if !test.allowed && !errors.Is(err, ErrForbidden) {
					t.Fatalf("expected forbidden, got %v", err)
				}
			}
		})
	}
}
