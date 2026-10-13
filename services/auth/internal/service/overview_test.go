package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"
)

type fakeOverviewStore struct {
	counts     repository.OverviewCounts
	series     []repository.OverviewPoint
	fromSeen   time.Time
	prevSeen   time.Time
	seriesFrom time.Time
	seriesTo   time.Time
}

func (f *fakeOverviewStore) Counts(_ context.Context, from, prevFrom time.Time) (repository.OverviewCounts, error) {
	f.fromSeen, f.prevSeen = from, prevFrom
	return f.counts, nil
}

func (f *fakeOverviewStore) Series(_ context.Context, from, to time.Time) ([]repository.OverviewPoint, error) {
	f.seriesFrom, f.seriesTo = from, to
	return f.series, nil
}

func newOverviewService(store *fakeOverviewStore) *service.AuthService {
	return service.NewForTest(newMockRepo(), time.Hour).WithOverview(store)
}

func TestAdminOverviewRequiresAdmin(t *testing.T) {
	store := &fakeOverviewStore{}
	svc := newOverviewService(store)

	// students and teachers must be refused
	if _, err := svc.AdminOverview(context.Background(), student, 28, "req", ""); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("student err = %v, want ErrForbidden", err)
	}
	if _, err := svc.AdminOverview(context.Background(), teacher, 28, "req", ""); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("teacher err = %v, want ErrForbidden", err)
	}
	if _, err := svc.AdminOverview(context.Background(), anonymous, 28, "req", ""); !errors.Is(err, service.ErrUnauthorized) {
		t.Fatalf("anonymous err = %v, want ErrUnauthorized", err)
	}
}

func TestAdminOverviewComputesDeltasAndWindows(t *testing.T) {
	store := &fakeOverviewStore{
		counts: repository.OverviewCounts{
			TotalUsers: 120, NewUsers: 30, NewUsersPrev: 20,
			ActiveUsers: 15, ActiveUsersPrev: 30,
			Assignments: 8, AuditEvents: 12, DeniedAttempts: 3,
		},
		series: []repository.OverviewPoint{{Date: "2026-10-13", NewUsers: 2, ActiveUsers: 5}},
	}
	svc := newOverviewService(store)

	view, err := svc.AdminOverview(context.Background(), adminActor, 7, "req", "")
	if err != nil {
		t.Fatalf("AdminOverview: %v", err)
	}
	if view.RangeDays != 7 {
		t.Fatalf("rangeDays = %d, want 7", view.RangeDays)
	}
	if view.Kpis.NewUsersDelta != 50 { // 30 vs 20
		t.Fatalf("newUsers delta = %d, want 50", view.Kpis.NewUsersDelta)
	}
	if view.Kpis.ActiveUsersDelta != -50 { // 15 vs 30
		t.Fatalf("activeUsers delta = %d, want -50", view.Kpis.ActiveUsersDelta)
	}
	if view.Kpis.DeniedAttempts != 3 || view.Kpis.TotalUsers != 120 {
		t.Fatalf("unexpected kpis: %+v", view.Kpis)
	}
	if len(view.Series) != 1 || view.Series[0].ActiveUsers != 5 {
		t.Fatalf("unexpected series: %+v", view.Series)
	}
	// Window must be the last 7 days ending today, and the comparison window the 7 before.
	if diff := store.seriesTo.Sub(store.seriesFrom); diff != 6*24*time.Hour {
		t.Fatalf("series window = %v, want 6 days", diff)
	}
	if diff := store.fromSeen.Sub(store.prevSeen); diff != 7*24*time.Hour {
		t.Fatalf("comparison window = %v, want 7 days", diff)
	}
}

func TestAdminOverviewClampsRange(t *testing.T) {
	store := &fakeOverviewStore{}
	svc := newOverviewService(store)

	view, err := svc.AdminOverview(context.Background(), adminActor, 9999, "req", "")
	if err != nil {
		t.Fatal(err)
	}
	if view.RangeDays != 90 {
		t.Fatalf("rangeDays = %d, want clamp to 90", view.RangeDays)
	}

	view, err = svc.AdminOverview(context.Background(), adminActor, 0, "req", "")
	if err != nil {
		t.Fatal(err)
	}
	if view.RangeDays != 28 {
		t.Fatalf("rangeDays = %d, want default 28", view.RangeDays)
	}
}

func TestAdminOverviewWithoutStoreIsRejected(t *testing.T) {
	svc := service.NewForTest(newMockRepo(), time.Hour)
	if _, err := svc.AdminOverview(context.Background(), adminActor, 28, "req", ""); err == nil {
		t.Fatal("expected an error when the overview store is not wired")
	}
}
