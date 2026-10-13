package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
)

type fakeStatsStore struct {
	series         []model.StudyDayPoint
	totals         [4]int
	due            int
	recent         []model.RecentSession
	cont           []model.ContinueItem
	owned          []model.OwnedSetStat
	summary        []model.ProgressSummaryRow
	summarySetIDs  []int64
	summaryUserIDs []int64
	fromSeen       time.Time
	toSeen         time.Time
}

func (f *fakeStatsStore) DailySeries(_ context.Context, _ int64, from, to time.Time) ([]model.StudyDayPoint, error) {
	f.fromSeen, f.toSeen = from, to
	return f.series, nil
}
func (f *fakeStatsStore) Totals(context.Context, int64, time.Time) (int, int, int, int, error) {
	return f.totals[0], f.totals[1], f.totals[2], f.totals[3], nil
}
func (f *fakeStatsStore) DueCards(context.Context, int64) (int, error) { return f.due, nil }
func (f *fakeStatsStore) RecentSessions(context.Context, int64, int) ([]model.RecentSession, error) {
	return f.recent, nil
}
func (f *fakeStatsStore) ContinueLearning(context.Context, int64, int) ([]model.ContinueItem, error) {
	return f.cont, nil
}
func (f *fakeStatsStore) OwnedSets(context.Context, int64, int) ([]model.OwnedSetStat, error) {
	return f.owned, nil
}
func (f *fakeStatsStore) ProgressByUsersAndSets(_ context.Context, setIDs, userIDs []int64) ([]model.ProgressSummaryRow, error) {
	f.summarySetIDs = setIDs
	f.summaryUserIDs = userIDs
	return f.summary, nil
}

func dayWithSessions(date string, sessions int) model.StudyDayPoint {
	return model.StudyDayPoint{Date: date, Sessions: sessions}
}

func TestStatsServiceRejectsAnonymous(t *testing.T) {
	svc := NewStatsService(&fakeStatsStore{})
	if _, err := svc.Get(context.Background(), 0, 28); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestStatsServiceComputesStreakAndAccuracy(t *testing.T) {
	store := &fakeStatsStore{
		series: []model.StudyDayPoint{
			dayWithSessions("2026-10-09", 1),
			dayWithSessions("2026-10-10", 2),
			dayWithSessions("2026-10-11", 1),
			dayWithSessions("2026-10-12", 0),
			dayWithSessions("2026-10-13", 0),
		},
		totals: [4]int{42, 7, 70, 49},
		due:    3,
	}
	svc := NewStatsService(store)
	svc.now = func() time.Time { return time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC) }

	got, err := svc.Get(context.Background(), 5, 28)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.StreakDays != 0 {
		t.Fatalf("streak = %d, want 0 (gap yesterday)", got.StreakDays)
	}
	if got.StudiedToday {
		t.Fatal("StudiedToday should be false when today has no sessions")
	}
	if got.SessionsTotal != 42 || got.SessionsInRange != 7 {
		t.Fatalf("totals = %+v", got)
	}
	if got.AccuracyPct != 70 {
		t.Fatalf("accuracy = %d, want 70", got.AccuracyPct)
	}
	if got.DueCards != 3 {
		t.Fatalf("dueCards = %d, want 3", got.DueCards)
	}
}

func TestStatsServiceStreakSurvivesUnfinishedToday(t *testing.T) {
	store := &fakeStatsStore{
		series: []model.StudyDayPoint{
			dayWithSessions("2026-10-11", 1),
			dayWithSessions("2026-10-12", 1),
			dayWithSessions("2026-10-13", 0),
		},
	}
	svc := NewStatsService(store)
	svc.now = func() time.Time { return time.Date(2026, 10, 13, 23, 0, 0, 0, time.UTC) }

	got, err := svc.Get(context.Background(), 5, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.StreakDays != 2 {
		t.Fatalf("streak = %d, want 2 (yesterday + day before)", got.StreakDays)
	}
}

func TestStatsServiceCountsTodayAsStreakAndClampsRange(t *testing.T) {
	store := &fakeStatsStore{
		series: []model.StudyDayPoint{
			dayWithSessions("2026-10-12", 1),
			dayWithSessions("2026-10-13", 3),
		},
	}
	svc := NewStatsService(store)
	svc.now = func() time.Time { return time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC) }

	got, err := svc.Get(context.Background(), 5, 500)
	if err != nil {
		t.Fatal(err)
	}
	if got.RangeDays != maxStatsDays {
		t.Fatalf("rangeDays = %d, want %d", got.RangeDays, maxStatsDays)
	}
	if got.StreakDays != 2 || !got.StudiedToday {
		t.Fatalf("streak = %d studiedToday = %v, want 2/true", got.StreakDays, got.StudiedToday)
	}
	// from must be inclusive of rangeDays days ending today.
	wantFrom := time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(maxStatsDays - 1))
	if !store.fromSeen.Equal(wantFrom) {
		t.Fatalf("from = %v, want %v", store.fromSeen, wantFrom)
	}
	if !store.toSeen.Equal(time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("to = %v", store.toSeen)
	}
}

func TestProgressSummaryValidatesAndForwardsIDs(t *testing.T) {
	store := &fakeStatsStore{summary: []model.ProgressSummaryRow{{UserID: 5, StudySetID: 9, Sessions: 2, BestScore: 8, Completed: true}}}
	svc := NewStatsService(store)

	if rows, err := svc.ProgressSummary(context.Background(), nil, []int64{5}); err != nil || rows != nil {
		t.Fatalf("empty set ids should be a no-op, got %v / %v", rows, err)
	}
	if _, err := svc.ProgressSummary(context.Background(), []int64{9}, nil); err != nil {
		t.Fatalf("empty user ids should be a no-op: %v", err)
	}

	rows, err := svc.ProgressSummary(context.Background(), []int64{9}, []int64{5})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].BestScore != 8 || !rows[0].Completed {
		t.Fatalf("unexpected summary rows: %+v", rows)
	}
	if len(store.summarySetIDs) != 1 || store.summarySetIDs[0] != 9 || len(store.summaryUserIDs) != 1 {
		t.Fatalf("repository did not receive the requested ids: %v / %v", store.summarySetIDs, store.summaryUserIDs)
	}
}

func TestProgressSummaryRejectsOversizedBatch(t *testing.T) {
	svc := NewStatsService(&fakeStatsStore{})
	ids := make([]int64, maxSummaryIDs+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	if _, err := svc.ProgressSummary(context.Background(), ids, []int64{1}); !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}

func TestClampStatsDays(t *testing.T) {
	cases := map[int]int{0: defaultStatsDays, -5: defaultStatsDays, 3: minStatsDays, 7: 7, 28: 28, 90: 90, 365: maxStatsDays}
	for in, want := range cases {
		if got := clampStatsDays(in); got != want {
			t.Fatalf("clampStatsDays(%d) = %d, want %d", in, got, want)
		}
	}
}
