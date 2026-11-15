package service

import (
	"context"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"slices"
	"testing"
)

func TestRoleEvaluationDenyWinsAndCannotImpersonateOwner(t *testing.T) {
	roles := append(ManagedClassRoles(), ClassRole{Key: "no-delete", Allow: []string{"class-document:Create", "bogus:Root"}, Deny: []string{"class-video:Delete"}})
	actions := EvaluateClassRoles([]string{"teacher", "no-delete", "owner"}, roles)
	if slices.Contains(actions, "class-video:Delete") || slices.Contains(actions, "class:Delete") || slices.Contains(actions, "bogus:Root") {
		t.Fatal(actions)
	}
	if !slices.Contains(actions, "class-document:Create") {
		t.Fatal("missing valid allow")
	}
	if len(EvaluateClassRoles([]string{"deleted-role"}, roles)) != 0 {
		t.Fatal("unknown role grants rights")
	}
}
func TestExplicitAssignmentReplacesLegacyTeacherAndIsClassScoped(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	access := &ClassAccess{DB: db, Classes: &fakeClassStore{class: &model.Class{ID: 10, OwnerUserID: 1}}, Members: &fakeMemberStore{roles: map[int64]string{2: "teacher"}}}
	mock.ExpectQuery("SELECT role_keys FROM class_member_access").WithArgs(int64(10), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"role_keys"}).AddRow(`["viewer","custom-restrict"]`))
	mock.ExpectQuery("SELECT key,name,description,allow_actions,deny_actions").WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"key", "name", "description", "allow", "deny"}).AddRow("custom-restrict", "No videos", "", `[]`, `["class-content:ViewVideo"]`))
	actions, err := access.Effective(context.Background(), 10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(actions, "class-content:ViewVideo") || slices.Contains(actions, "class-video:Delete") {
		t.Fatal("legacy teacher leaked privileges", actions)
	}
	if !slices.Contains(actions, "class-document:Read") {
		t.Fatal(actions)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestClassAccessOwnerLegacyFallbackAndDatabaseFailure(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	access := &ClassAccess{DB: db, Classes: &fakeClassStore{class: &model.Class{ID: 10, OwnerUserID: 1}}, Members: &fakeMemberStore{roles: map[int64]string{2: "student"}}}
	if err := access.Require(context.Background(), 10, 1, "class-access:Manage"); err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT role_keys").WithArgs(int64(10), int64(2)).WillReturnError(sql.ErrNoRows)
	if err := access.Require(context.Background(), 10, 2, "class-document:Create"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT role_keys").WithArgs(int64(10), int64(2)).WillReturnError(errors.New("db unavailable"))
	if err := access.Require(context.Background(), 10, 2, "class-document:Read"); err == nil {
		t.Fatal("database failure granted access")
	}
	if err := access.Require(context.Background(), 10, 99, "class-document:Read"); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
	if err := access.Require(context.Background(), 10, 0, "class-document:Read"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
