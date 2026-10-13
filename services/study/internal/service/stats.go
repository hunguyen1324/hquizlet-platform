// Package service – stats.go implements dashboard statistics for /v1/me/study-stats.
package service

import (
	"context"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
)

const (
	defaultStatsDays = 28
	minStatsDays     = 7
	maxStatsDays     = 90
	recentLimit      = 5
	continueLimit    = 5
	ownedSetLimit    = 10
)

// StatsService builds the personal dashboard payload from learning activity.
type StatsService struct {
	stats repository.StudyStatsStore
	now   func() time.Time
}

func NewStatsService(stats repository.StudyStatsStore) *StatsService {
	return &StatsService{stats: stats, now: time.Now}
}

// Get returns dashboard statistics for userID over the requested window.
// days is clamped to [7, 90] with a 28-day default.
func (s *StatsService) Get(ctx context.Context, userID int64, days int) (model.StudyStats, error) {
	if userID <= 0 {
		return model.StudyStats{}, ErrUnauthorized
	}
	days = clampStatsDays(days)

	now := s.now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := today.AddDate(0, 0, -(days - 1))

	series, err := s.stats.DailySeries(ctx, userID, from, today)
	if err != nil {
		return model.StudyStats{}, err
	}
	sessionsTotal, sessionsInRange, cardsInRange, correctInRange, err := s.stats.Totals(ctx, userID, from)
	if err != nil {
		return model.StudyStats{}, err
	}
	due, err := s.stats.DueCards(ctx, userID)
	if err != nil {
		return model.StudyStats{}, err
	}
	recent, err := s.stats.RecentSessions(ctx, userID, recentLimit)
	if err != nil {
		return model.StudyStats{}, err
	}
	continued, err := s.stats.ContinueLearning(ctx, userID, continueLimit)
	if err != nil {
		return model.StudyStats{}, err
	}
	owned, err := s.stats.OwnedSets(ctx, userID, ownedSetLimit)
	if err != nil {
		return model.StudyStats{}, err
	}

	return model.StudyStats{
		RangeDays:        days,
		StreakDays:       streakDays(series),
		StudiedToday:     len(series) > 0 && series[len(series)-1].Sessions > 0,
		SessionsTotal:    sessionsTotal,
		SessionsInRange:  sessionsInRange,
		CardsInRange:     cardsInRange,
		CorrectInRange:   correctInRange,
		AccuracyPct:      accuracyPct(correctInRange, cardsInRange),
		DueCards:         due,
		Series:           series,
		RecentSessions:   recent,
		ContinueLearning: continued,
		OwnedSets:        owned,
	}, nil
}

// maxSummaryIDs bounds a single internal progress request.
const maxSummaryIDs = 200

// ProgressSummary returns per-(user, study set) aggregates for the class dashboard.
// It is an internal API: the caller (class service) is trusted and never a browser.
func (s *StatsService) ProgressSummary(ctx context.Context, studySetIDs, userIDs []int64) ([]model.ProgressSummaryRow, error) {
	if len(studySetIDs) == 0 || len(userIDs) == 0 {
		return nil, nil
	}
	if len(studySetIDs) > maxSummaryIDs || len(userIDs) > maxSummaryIDs {
		return nil, ErrValidation
	}
	return s.stats.ProgressByUsersAndSets(ctx, studySetIDs, userIDs)
}

func clampStatsDays(days int) int {
	if days <= 0 {
		return defaultStatsDays
	}
	if days < minStatsDays {
		return minStatsDays
	}
	if days > maxStatsDays {
		return maxStatsDays
	}
	return days
}

// streakDays counts consecutive active days ending today (or yesterday, so a user
// who has not studied yet today keeps their streak).
func streakDays(series []model.StudyDayPoint) int {
	if len(series) == 0 {
		return 0
	}
	i := len(series) - 1
	if series[i].Sessions == 0 {
		i-- // allow a gap for today
	}
	streak := 0
	for ; i >= 0; i-- {
		if series[i].Sessions == 0 {
			break
		}
		streak++
	}
	return streak
}

func accuracyPct(correct, total int) int {
	if total <= 0 {
		return 0
	}
	return int(float64(correct)*100/float64(total) + 0.5)
}
