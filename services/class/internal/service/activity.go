package service

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/client"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
)

// ActivityService handles activity feed aggregation.
type ActivityService struct {
	activities  repository.ActivityStore
	studyClient *client.StudyClient
}

// NewActivityService creates a new ActivityService.
func NewActivityService(activities repository.ActivityStore, studyClient *client.StudyClient) *ActivityService {
	return &ActivityService{
		activities:  activities,
		studyClient: studyClient,
	}
}

// RecordEvent inserts an activity event.
func (s *ActivityService) RecordEvent(ctx context.Context, event model.ActivityEvent) error {
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	return s.activities.Create(ctx, event)
}

// GetFeed returns the aggregated activity feed for a user.
func (s *ActivityService) GetFeed(ctx context.Context, userID int64, limit int, cursorStr string) (*model.ActivityFeedResponse, error) {
	if limit <= 0 {
		limit = 20
	}

	var cursor *model.ActivityCursor
	if cursorStr != "" {
		var err error
		cursor, err = repository.DecodeCursor(cursorStr)
		if err != nil {
			return nil, err
		}
	}

	// 1. Query class activity events
	events, err := s.activities.ListByUser(ctx, userID, limit, cursor)
	if err != nil {
		return nil, err
	}

	// 2. Try to get study progress from Study service
	var studyItems []client.StudyProgressItem
	if s.studyClient != nil {
		studyItems, err = s.studyClient.GetRecentProgress(ctx, userID, limit)
		if err != nil {
			// Study service unavailable — log and continue with partial result
			log.Printf("[activity] warning: study service unavailable: %v", err)
		}
	}

	// 3. Merge events into ActivityItems
	items := make([]model.ActivityItem, 0, len(events))
	for _, e := range events {
		item := model.ActivityItem{
			ID:         e.ID,
			EventType:  e.EventType,
			EntityType: e.EntityType,
			EntityID:   e.EntityID,
			ClassID:    e.ClassID,
			OccurredAt: e.OccurredAt,
		}
		// Parse metadata JSON
		if e.Metadata != nil {
			var meta map[string]any
			if err := json.Unmarshal(e.Metadata, &meta); err == nil {
				item.Metadata = meta
			}
		}
		items = append(items, item)
	}

	// 4. Append study progress items as activity events
	for _, sp := range studyItems {
		item := model.ActivityItem{
			ID:         sp.ID,
			EventType:  "study.progress",
			EntityType: "study_set",
			OccurredAt: parseTime(sp.CreatedAt),
			Metadata: map[string]any{
				"studySetId": sp.StudySetID,
				"mode":       sp.Mode,
				"score":      sp.Score,
				"total":      sp.Total,
			},
		}
		items = append(items, item)
	}

	// 5. Sort by occurred_at DESC (study items may be out of order)
	sortActivityItems(items)

	// 6. Apply limit
	if len(items) > limit {
		items = items[:limit]
	}

	// 7. Build cursor
	var nextCursor string
	var hasMore bool
	if len(items) > 0 {
		lastItem := items[len(items)-1]
		nextCursor = repository.EncodeCursor(lastItem.OccurredAt, lastItem.ID)
		// Check if there might be more (simplified check)
		hasMore = len(items) == limit
	}

	return &model.ActivityFeedResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// sortActivityItems sorts by OccurredAt DESC.
func sortActivityItems(items []model.ActivityItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].OccurredAt.After(items[j-1].OccurredAt); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}
