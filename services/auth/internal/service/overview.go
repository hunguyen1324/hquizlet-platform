package service

import (
	"context"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
)

const (
	defaultOverviewRange = 28
	maxOverviewRange     = 90
)

// OverviewKpis is the KPI block of the admin overview.
type OverviewKpis struct {
	TotalUsers      int `json:"totalUsers"`
	NewUsers        int `json:"newUsers"`
	NewUsersDelta   int `json:"newUsersDeltaPct"`
	ActiveUsers     int `json:"activeUsers"`
	ActiveUsersDelta int `json:"activeUsersDeltaPct"`
	Assignments     int `json:"assignments"`
	AuditEvents     int `json:"auditEvents"`
	DeniedAttempts  int `json:"deniedAttempts"`
}

// OverviewPoint is one day of the admin chart.
type OverviewPoint struct {
	Date        string `json:"date"`
	NewUsers    int    `json:"newUsers"`
	ActiveUsers int    `json:"activeUsers"`
}

// AdminOverview is the payload for GET /v1/admin/overview.
type AdminOverview struct {
	RangeDays int             `json:"rangeDays"`
	Kpis      OverviewKpis    `json:"kpis"`
	Series    []OverviewPoint `json:"series"`
}

// AdminOverview reads platform-wide activity for the admin console.
// Requires dashboard:ViewAdmin; the caller is never trusted to scope itself.
func (s *AuthService) AdminOverview(ctx context.Context, actor authz.Principal, rangeDays int, requestID, ip string) (AdminOverview, error) {
	if err := requirePlatform(actor, authz.ActionDashboardViewAdmin); err != nil {
		s.auditDenied(ctx, actor, string(authz.ActionDashboardViewAdmin), "hq:platform", requestID, ip)
		return AdminOverview{}, err
	}
	if s.overview == nil {
		return AdminOverview{}, ErrUnauthorized
	}
	days := clampOverviewRange(rangeDays)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	from := today.AddDate(0, 0, -(days - 1))
	prevFrom := from.AddDate(0, 0, -days)

	counts, err := s.overview.Counts(ctx, from, prevFrom)
	if err != nil {
		return AdminOverview{}, err
	}
	series, err := s.overview.Series(ctx, from, today)
	if err != nil {
		return AdminOverview{}, err
	}
	points := make([]OverviewPoint, 0, len(series))
	for _, p := range series {
		points = append(points, OverviewPoint{Date: p.Date, NewUsers: p.NewUsers, ActiveUsers: p.ActiveUsers})
	}

	return AdminOverview{
		RangeDays: days,
		Kpis: OverviewKpis{
			TotalUsers:       counts.TotalUsers,
			NewUsers:         counts.NewUsers,
			NewUsersDelta:    deltaPct(counts.NewUsers, counts.NewUsersPrev),
			ActiveUsers:      counts.ActiveUsers,
			ActiveUsersDelta: deltaPct(counts.ActiveUsers, counts.ActiveUsersPrev),
			Assignments:      counts.Assignments,
			AuditEvents:      counts.AuditEvents,
			DeniedAttempts:   counts.DeniedAttempts,
		},
		Series: points,
	}, nil
}

func clampOverviewRange(days int) int {
	if days <= 0 {
		return defaultOverviewRange
	}
	if days > maxOverviewRange {
		return maxOverviewRange
	}
	return days
}

// deltaPct returns the percentage change vs the previous window (0 when there is no baseline).
func deltaPct(current, previous int) int {
	if previous == 0 {
		if current == 0 {
			return 0
		}
		return 100
	}
	return int((float64(current-previous) / float64(previous) * 100))
}
