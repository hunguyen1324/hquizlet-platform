package service

import (
	"context"
	"encoding/json"
	"log"
	"sort"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/client"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
)

// progressSummarizer is the study-service surface the dashboard needs (implemented by
// *client.StudyClient; narrowed to an interface so tests can inject a fake).
type progressSummarizer interface {
	GetProgressSummary(ctx context.Context, studySetIDs, userIDs []int64) ([]client.StudyProgressSummaryRow, error)
}

// userDirectory resolves member display names (implemented by *client.UserClient).
type userDirectory interface {
	Lookup(ctx context.Context, ids []int64) (map[int64]client.UserDirectoryEntry, error)
}

// ClassDashboardService aggregates class progress for GET /v1/classes/{id}/dashboard.
type ClassDashboardService struct {
	classes     repository.ClassStore
	members     repository.MemberStore
	studySets   repository.ClassStudySetStore
	dashboard   repository.DashboardStore
	studyClient progressSummarizer
	userClient  userDirectory
	now         func() time.Time
}

func NewClassDashboardService(
	classes repository.ClassStore,
	members repository.MemberStore,
	studySets repository.ClassStudySetStore,
	dashboard repository.DashboardStore,
	studyClient progressSummarizer,
	userClient userDirectory,
) *ClassDashboardService {
	return &ClassDashboardService{
		classes: classes, members: members, studySets: studySets, dashboard: dashboard,
		studyClient: studyClient, userClient: userClient, now: time.Now,
	}
}

// Get returns dashboard data scoped to the caller's role in the class.
func (s *ClassDashboardService) Get(ctx context.Context, p authz.Principal, classID int64) (*model.ClassDashboard, error) {
	class, role, err := s.authorizeAndResolve(ctx, p, classID, authz.ActionClassViewDashboard)
	if err != nil {
		return nil, err
	}

	members, err := s.members.ListByClass(ctx, classID)
	if err != nil {
		return nil, err
	}
	sets, err := s.studySets.List(ctx, classID)
	if err != nil {
		return nil, err
	}
	if lookup, ok := s.studyClient.(interface {
		GetStudySet(context.Context, int64) (*client.StudySetInfo, error)
	}); ok {
		for _, set := range sets {
			if info, err := lookup.GetStudySet(ctx, set.StudySetID); err == nil && info != nil {
				set.Title = info.Title
			}
		}
	}
	showLeaderboard, err := s.dashboard.LeaderboardEnabled(ctx, classID)
	if err != nil {
		return nil, err
	}

	canViewAll := canViewOthersProgress(p, classID, role)
	setIDs := make([]int64, 0, len(sets))
	for _, set := range sets {
		setIDs = append(setIDs, set.StudySetID)
	}

	out := &model.ClassDashboard{
		ClassID:         classID,
		Students:        []model.DashboardStudentRow{},
		RecentActivity:  []model.ActivityItem{},
		ClassName:       class.Name,
		ViewerRole:      role,
		CanViewAll:      canViewAll,
		ShowLeaderboard: showLeaderboard,
		MemberCount:     len(members),
	}

	// Whose progress is fetched from the study service.
	scopeIDs := []int64{p.UserID}
	if canViewAll {
		scopeIDs = memberIDs(members)
	}

	rows, err := s.studyClient.GetProgressSummary(ctx, setIDs, scopeIDs)
	if err != nil {
		// Degrade instead of failing the whole dashboard: classes/sets still render.
		log.Printf("[class] progress summary unavailable: %v", err)
		rows = nil
	}

	perUser := aggregateByUser(rows)
	out.ActiveMembers7d = countActiveSince(perUser, s.now().UTC().AddDate(0, 0, -7))

	// Completion is measured against the learners who can actually do the work: class
	// students for a teacher view, and just the caller for a student view.
	denominator := countStudents(members)
	if !canViewAll {
		denominator = 1
	} else if denominator == 0 {
		denominator = len(members)
	}
	out.AssignedSets = buildSetStats(sets, rows, denominator)

	names := s.lookupNames(ctx, scopeIDs)

	if canViewAll {
		out.Students = buildStudentRows(members, perUser, names, true)
	} else {
		out.MyProgress = ownProgress(p.UserID, perUser, names)
		if showLeaderboard {
			ranking, err := s.studyClient.GetProgressSummary(ctx, setIDs, memberIDs(members))
			if err == nil {
				out.Students = buildStudentRows(members, aggregateByUser(ranking), s.lookupNames(ctx, memberIDs(members)), true)
				for i := range out.Students {
					out.Students[i].Email = ""
					out.Students[i].Sessions = 0
					out.Students[i].CompletedSets = 0
					out.Students[i].LastActivityAt = nil
				}
			}
		}
	}

	if canViewAll {
		out.RecentActivity = s.recentActivity(ctx, classID)
	}
	return out, nil
}

// SetLeaderboard toggles the per-class leaderboard visibility (owner/teacher only).
func (s *ClassDashboardService) SetLeaderboard(ctx context.Context, p authz.Principal, classID int64, enabled bool) error {
	// Uses class:Update so the same actors who edit a class control visibility.
	if _, _, err := s.authorizeAndResolve(ctx, p, classID, authz.ActionClassContentAssign); err != nil {
		return err
	}
	return s.dashboard.SetLeaderboard(ctx, classID, enabled)
}

// --- helpers ---

func (s *ClassDashboardService) authorizeAndResolve(ctx context.Context, p authz.Principal, classID int64, action authz.Action) (*model.Class, string, error) {
	if p.UserID <= 0 {
		return nil, "", ErrUnauthorized
	}
	class, err := s.classes.GetByID(ctx, classID)
	if err != nil {
		return nil, "", err
	}
	role := resolveClassRole(ctx, s.members, class, classID, p.UserID)
	if role == "" {
		if !authz.Can(p, action, authz.Resource{Type: "class", ID: classID, ClassID: classID, OwnerUserID: class.OwnerUserID}) {
			return nil, "", ErrForbidden
		}
		role = "admin"
	}
	p.ClassID = classID
	p.ClassRole = role
	res := authz.Resource{Type: "class", ID: classID, ClassID: classID, OwnerUserID: class.OwnerUserID}
	if !authz.Can(p, action, res) {
		return nil, "", ErrForbidden
	}
	return class, role, nil
}

// canViewOthersProgress asks the policy engine whether this principal may read a *different*
// member's progress (OwnerUserID is set to a sentinel that is never the caller).
func canViewOthersProgress(p authz.Principal, classID int64, role string) bool {
	probe := p
	probe.ClassID = classID
	probe.ClassRole = role
	res := authz.Resource{Type: "class", ID: classID, ClassID: classID, OwnerUserID: -1}
	return authz.Can(probe, authz.ActionClassMemberViewProgress, res)
}

// countStudents counts members whose role is "student".
func countStudents(members []*model.ClassMember) int {
	n := 0
	for _, m := range members {
		if m.Role == "student" {
			n++
		}
	}
	return n
}

func memberIDs(members []*model.ClassMember) []int64 {
	out := make([]int64, 0, len(members))
	for _, m := range members {
		out = append(out, m.UserID)
	}
	return out
}

type userAggregate struct {
	sessions    int
	completed   int
	weightedAvg float64
	samples     int
	last        string
}

func aggregateByUser(rows []client.StudyProgressSummaryRow) map[int64]*userAggregate {
	out := make(map[int64]*userAggregate, len(rows))
	for _, r := range rows {
		agg := out[r.UserID]
		if agg == nil {
			agg = &userAggregate{}
			out[r.UserID] = agg
		}
		agg.sessions += r.Sessions
		if r.Completed {
			agg.completed++
		}
		agg.weightedAvg += float64(r.AvgScorePct)
		agg.samples++
		if r.LastActivityAt != nil && *r.LastActivityAt > agg.last {
			agg.last = *r.LastActivityAt
		}
	}
	return out
}

func (a *userAggregate) avgScorePct() int {
	if a == nil || a.samples == 0 {
		return 0
	}
	return int(a.weightedAvg/float64(a.samples) + 0.5)
}

func countActiveSince(perUser map[int64]*userAggregate, since time.Time) int {
	cutoff := since.Format(time.RFC3339)
	n := 0
	for _, agg := range perUser {
		if agg == nil {
			continue
		}
		if agg.sessions > 0 && agg.last >= cutoff {
			n++
		}
	}
	return n
}

// buildSetStats aggregates the raw (user, set) rows into per-set statistics.
func buildSetStats(sets []*model.ClassStudySet, rows []client.StudyProgressSummaryRow, denominator int) []model.DashboardSetStat {
	type setAgg struct {
		sessions  int
		completed int
		scoreSum  int
		samples   int
	}
	bySet := make(map[int64]*setAgg, len(sets))
	learners := make(map[int64]map[int64]struct{}, len(sets))
	for _, r := range rows {
		agg := bySet[r.StudySetID]
		if agg == nil {
			agg = &setAgg{}
			bySet[r.StudySetID] = agg
		}
		agg.sessions += r.Sessions
		agg.scoreSum += r.AvgScorePct
		agg.samples++
		if r.Completed {
			agg.completed++
		}
		if r.Sessions > 0 {
			if learners[r.StudySetID] == nil {
				learners[r.StudySetID] = map[int64]struct{}{}
			}
			learners[r.StudySetID][r.UserID] = struct{}{}
		}
	}

	out := make([]model.DashboardSetStat, 0, len(sets))
	for _, set := range sets {
		stat := model.DashboardSetStat{
			DueAt:      set.DueAt,
			StudySetID: set.StudySetID,
			Title:      set.Title,
			Learners:   len(learners[set.StudySetID]),
		}
		if agg := bySet[set.StudySetID]; agg != nil {
			stat.Sessions = agg.sessions
			if agg.samples > 0 {
				stat.AvgScorePct = int(float64(agg.scoreSum)/float64(agg.samples) + 0.5)
			}
			if denominator > 0 {
				stat.CompletionPct = int(float64(agg.completed)*100/float64(denominator) + 0.5)
			}
		}
		out = append(out, stat)
	}
	return out
}

func buildStudentRows(members []*model.ClassMember, perUser map[int64]*userAggregate, names map[int64]client.UserDirectoryEntry, studentsOnly bool) []model.DashboardStudentRow {
	rows := make([]model.DashboardStudentRow, 0, len(members))
	for _, m := range members {
		if studentsOnly && m.Role != "student" {
			continue
		}
		agg := perUser[m.UserID]
		if agg == nil {
			agg = &userAggregate{}
		}
		row := model.DashboardStudentRow{
			UserID:        m.UserID,
			Role:          m.Role,
			Sessions:      agg.sessions,
			CompletedSets: agg.completed,
			AvgScorePct:   agg.avgScorePct(),
		}
		if agg.last != "" {
			last := agg.last
			row.LastActivityAt = &last
		}
		if entry, ok := names[m.UserID]; ok {
			row.Name = entry.Name
			row.Email = entry.Email
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].AvgScorePct != rows[j].AvgScorePct {
			return rows[i].AvgScorePct > rows[j].AvgScorePct
		}
		return rows[i].Sessions > rows[j].Sessions
	})
	return rows
}

func ownProgress(userID int64, perUser map[int64]*userAggregate, names map[int64]client.UserDirectoryEntry) *model.DashboardStudentRow {
	agg := perUser[userID]
	if agg == nil {
		agg = &userAggregate{}
	}
	row := model.DashboardStudentRow{
		UserID:        userID,
		Sessions:      agg.sessions,
		CompletedSets: agg.completed,
		AvgScorePct:   agg.avgScorePct(),
	}
	if agg.last != "" {
		last := agg.last
		row.LastActivityAt = &last
	}
	if entry, ok := names[userID]; ok {
		row.Name = entry.Name
	}
	return &row
}

// lookupNames resolves display names best-effort: the dashboard still renders without them.
func (s *ClassDashboardService) lookupNames(ctx context.Context, ids []int64) map[int64]client.UserDirectoryEntry {
	if s.userClient == nil || len(ids) == 0 {
		return map[int64]client.UserDirectoryEntry{}
	}
	names, err := s.userClient.Lookup(ctx, ids)
	if err != nil {
		log.Printf("[class] user directory unavailable: %v", err)
		return map[int64]client.UserDirectoryEntry{}
	}
	return names
}

func (s *ClassDashboardService) recentActivity(ctx context.Context, classID int64) []model.ActivityItem {
	events, err := s.dashboard.ListClassActivity(ctx, classID, 15)
	if err != nil {
		log.Printf("[class] activity unavailable: %v", err)
		return []model.ActivityItem{}
	}
	items := make([]model.ActivityItem, 0, len(events))
	for _, e := range events {
		item := model.ActivityItem{
			ID: e.ID, EventType: e.EventType, EntityType: e.EntityType,
			EntityID: e.EntityID, ClassID: e.ClassID, OccurredAt: e.OccurredAt,
		}
		if len(e.Metadata) > 0 {
			var meta map[string]any
			if json.Unmarshal(e.Metadata, &meta) == nil {
				item.Metadata = meta
			}
		}
		items = append(items, item)
	}
	return items
}

// resolveClassRole returns the member role, falling back to "owner" for the class owner.
func resolveClassRole(ctx context.Context, members repository.MemberStore, class *model.Class, classID, userID int64) string {
	role, err := members.GetRole(ctx, classID, userID)
	if err == nil && role != "" {
		return role
	}
	if class != nil && class.OwnerUserID == userID {
		return "owner"
	}
	return ""
}
