package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/client"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

// --- fakes ---

type fakeClassStore struct{ class *model.Class }

func (f *fakeClassStore) Create(context.Context, int64, model.CreateClassInput) (*model.Class, error) {
	return nil, errors.New("not used")
}
func (f *fakeClassStore) GetByID(context.Context, int64) (*model.Class, error) { return f.class, nil }
func (f *fakeClassStore) GetByInviteCode(context.Context, string) (*model.Class, error) {
	return nil, errors.New("not used")
}
func (f *fakeClassStore) ListByUserID(context.Context, int64) ([]*model.ClassSummary, error) {
	return nil, nil
}
func (f *fakeClassStore) Update(context.Context, int64, model.UpdateClassInput) (*model.Class, error) {
	return nil, errors.New("not used")
}
func (f *fakeClassStore) Delete(context.Context, int64) error { return nil }
func (f *fakeClassStore) ResetInviteCode(context.Context, int64, string) error {
	return nil
}

type fakeMemberStore struct{ roles map[int64]string }

func (f *fakeMemberStore) Add(context.Context, int64, int64, string) (*model.ClassMember, error) {
	return nil, errors.New("not used")
}
func (f *fakeMemberStore) ListByClass(context.Context, int64) ([]*model.ClassMember, error) {
	out := make([]*model.ClassMember, 0, len(f.roles))
	for userID, role := range f.roles {
		out = append(out, &model.ClassMember{UserID: userID, Role: role})
	}
	return out, nil
}
func (f *fakeMemberStore) GetRole(_ context.Context, _, userID int64) (string, error) {
	return f.roles[userID], nil
}
func (f *fakeMemberStore) UpdateRole(context.Context, int64, int64, string) error { return nil }
func (f *fakeMemberStore) Remove(context.Context, int64, int64) error             { return nil }
func (f *fakeMemberStore) CountByClass(context.Context, int64) (int, error) {
	return len(f.roles), nil
}

type fakeStudySetStore struct{ sets []*model.ClassStudySet }

func (f *fakeStudySetStore) Add(context.Context, int64, int64, int64) error { return nil }
func (f *fakeStudySetStore) List(context.Context, int64) ([]*model.ClassStudySet, error) {
	return f.sets, nil
}
func (f *fakeStudySetStore) Remove(context.Context, int64, int64) error { return nil }
func (f *fakeStudySetStore) CountByClass(context.Context, int64) (int, error) {
	return len(f.sets), nil
}

type fakeDashboardStore struct {
	leaderboard   bool
	setCalls      []bool
	activityItems []*model.ActivityEvent
}

func (f *fakeDashboardStore) LeaderboardEnabled(context.Context, int64) (bool, error) {
	return f.leaderboard, nil
}
func (f *fakeDashboardStore) SetLeaderboard(_ context.Context, _ int64, enabled bool) error {
	f.setCalls = append(f.setCalls, enabled)
	f.leaderboard = enabled
	return nil
}
func (f *fakeDashboardStore) ListClassActivity(context.Context, int64, int) ([]*model.ActivityEvent, error) {
	return f.activityItems, nil
}

type fakeSummarizer struct {
	rows         []client.StudyProgressSummaryRow
	lastUserIDs  []int64
	lastStudyIDs []int64
	err          error
}

func (f *fakeSummarizer) GetProgressSummary(_ context.Context, studySetIDs, userIDs []int64) ([]client.StudyProgressSummaryRow, error) {
	f.lastStudyIDs, f.lastUserIDs = studySetIDs, userIDs
	if f.err != nil {
		return nil, f.err
	}
	// Mirror the real service: return only rows for the requested users.
	want := map[int64]struct{}{}
	for _, id := range userIDs {
		want[id] = struct{}{}
	}
	var out []client.StudyProgressSummaryRow
	for _, r := range f.rows {
		if _, ok := want[r.UserID]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

type fakeDirectory struct {
	entries map[int64]client.UserDirectoryEntry
	lastIDs []int64
}

func (f *fakeDirectory) Lookup(_ context.Context, ids []int64) (map[int64]client.UserDirectoryEntry, error) {
	f.lastIDs = ids
	out := map[int64]client.UserDirectoryEntry{}
	for _, id := range ids {
		if e, ok := f.entries[id]; ok {
			out[id] = e
		}
	}
	return out, nil
}

const testClassID = 10

func newDashboardFixture(leaderboard bool) (*ClassDashboardService, *fakeDashboardStore, *fakeSummarizer) {
	class := &model.Class{ID: testClassID, OwnerUserID: 1, Name: "Lớp 12A"}
	classStore := &fakeClassStore{class: class}
	members := &fakeMemberStore{roles: map[int64]string{1: "owner", 2: "student", 3: "student"}}
	sets := &fakeStudySetStore{sets: []*model.ClassStudySet{{ClassID: testClassID, StudySetID: 100, Title: "TOEIC"}}}
	dash := &fakeDashboardStore{leaderboard: leaderboard}
	summary := &fakeSummarizer{rows: []client.StudyProgressSummaryRow{
		{UserID: 2, StudySetID: 100, Sessions: 3, BestScore: 9, AvgScorePct: 90, Completed: true},
		{UserID: 3, StudySetID: 100, Sessions: 2, BestScore: 5, AvgScorePct: 50, Completed: false},
	}}
	dir := &fakeDirectory{entries: map[int64]client.UserDirectoryEntry{
		1: {ID: 1, Name: "Cô giáo", Email: "owner@example.com"},
		2: {ID: 2, Name: "An", Email: "an@example.com"},
		3: {ID: 3, Name: "Bình", Email: "binh@example.com"},
	}}
	svc := NewClassDashboardService(classStore, members, sets, dash, summary, dir)
	svc.now = func() time.Time { return time.Now().UTC() }
	return svc, dash, summary
}

func teacherPrincipal() authz.Principal {
	return authz.Principal{UserID: 1, PlatformRoles: []string{authz.RoleTeacher}}
}

func studentPrincipal() authz.Principal {
	return authz.Principal{UserID: 2, PlatformRoles: []string{authz.RoleStudent}}
}

func TestDashboardOwnerSeesWholeClass(t *testing.T) {
	svc, _, _ := newDashboardFixture(false)

	view, err := svc.Get(context.Background(), teacherPrincipal(), testClassID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !view.CanViewAll {
		t.Fatal("owner must see the whole class")
	}
	// The student table lists learners only (the owner is not a learner).
	if len(view.Students) != 2 {
		t.Fatalf("students = %d, want 2", len(view.Students))
	}
	// Sorted by average score descending.
	if view.Students[0].UserID != 2 || view.Students[0].AvgScorePct != 90 {
		t.Fatalf("unexpected ordering: %+v", view.Students)
	}
	if view.Students[0].Name != "An" {
		t.Fatalf("expected resolved name, got %q", view.Students[0].Name)
	}
	if view.AssignedSets[0].CompletionPct != 50 { // 1 of 2 students completed
		t.Fatalf("completionPct = %d, want 50", view.AssignedSets[0].CompletionPct)
	}
	if view.MemberCount != 3 {
		t.Fatalf("memberCount = %d, want 3", view.MemberCount)
	}
}

func TestDashboardStudentOnlySeesOwnProgress(t *testing.T) {
	svc, _, summary := newDashboardFixture(false)

	view, err := svc.Get(context.Background(), studentPrincipal(), testClassID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.CanViewAll {
		t.Fatal("a class student must not see class-wide progress")
	}
	if len(summary.lastUserIDs) != 1 || summary.lastUserIDs[0] != 2 {
		t.Fatalf("study service was queried for %v, want only the caller", summary.lastUserIDs)
	}
	if view.MyProgress == nil || view.MyProgress.AvgScorePct != 90 {
		t.Fatalf("myProgress = %+v", view.MyProgress)
	}
	// No leaderboard => no peer rows at all.
	if len(view.Students) != 0 {
		t.Fatalf("student saw %d peer rows without a leaderboard: %+v", len(view.Students), view.Students)
	}
	for _, row := range view.Students {
		if row.UserID == 3 {
			t.Fatal("student must never receive another student's progress")
		}
	}
	// Set stats reflect only the caller (1 learner denominator).
	if view.AssignedSets[0].Sessions != 3 || view.AssignedSets[0].Learners != 1 {
		t.Fatalf("own set stats leaked class data: %+v", view.AssignedSets[0])
	}
}

func TestDashboardStudentLeaderboardWhenEnabled(t *testing.T) {
	svc, _, _ := newDashboardFixture(true)

	view, err := svc.Get(context.Background(), studentPrincipal(), testClassID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(view.Students) != 2 {
		t.Fatalf("leaderboard rows = %d, want 2 learners", len(view.Students))
	}
	if view.MyProgress == nil {
		t.Fatal("student should still get their own progress row")
	}
}

func TestDashboardNonMemberIsForbidden(t *testing.T) {
	svc, _, _ := newDashboardFixture(false)
	outsider := authz.Principal{UserID: 99, PlatformRoles: []string{authz.RoleStudent}}

	if _, err := svc.Get(context.Background(), outsider, testClassID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	if _, err := svc.Get(context.Background(), authz.Principal{}, testClassID); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("anonymous err = %v, want ErrUnauthorized", err)
	}
}

func TestDashboardDegradesWhenStudyServiceIsDown(t *testing.T) {
	svc, _, summary := newDashboardFixture(false)
	summary.err = errors.New("study service unavailable")

	view, err := svc.Get(context.Background(), teacherPrincipal(), testClassID)
	if err != nil {
		t.Fatalf("dashboard must degrade, not fail: %v", err)
	}
	if len(view.AssignedSets) != 1 {
		t.Fatalf("assigned sets should still render: %+v", view.AssignedSets)
	}
	if view.AssignedSets[0].Sessions != 0 {
		t.Fatalf("expected zeroed stats when study is down: %+v", view.AssignedSets[0])
	}
}

func TestSetLeaderboardRequiresEditPermission(t *testing.T) {
	svc, dash, _ := newDashboardFixture(false)

	if err := svc.SetLeaderboard(context.Background(), studentPrincipal(), testClassID, true); !errors.Is(err, ErrForbidden) {
		t.Fatalf("student err = %v, want ErrForbidden", err)
	}
	if len(dash.setCalls) != 0 {
		t.Fatalf("student changed the leaderboard flag: %v", dash.setCalls)
	}
	if err := svc.SetLeaderboard(context.Background(), teacherPrincipal(), testClassID, true); err != nil {
		t.Fatalf("owner toggle failed: %v", err)
	}
	if len(dash.setCalls) != 1 || !dash.setCalls[0] {
		t.Fatalf("unexpected setCalls: %v", dash.setCalls)
	}
}
