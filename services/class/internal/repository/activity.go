package repository

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/model"
)

// ActivityRepository implements model.ActivityRepository backed by PostgreSQL.
type ActivityRepository struct {
	db *sql.DB
}

func NewActivityRepository(db *sql.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

// Create inserts a new activity event.
func (r *ActivityRepository) Create(ctx context.Context, event model.ActivityEvent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO activity_events (user_id, event_type, entity_type, entity_id, class_id, metadata, occurred_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, event.UserID, event.EventType, event.EntityType, event.EntityID, event.ClassID, event.Metadata, event.OccurredAt)
	return err
}

// ListByUser returns activity events for a user, with optional cursor pagination.
func (r *ActivityRepository) ListByUser(ctx context.Context, userID int64, limit int, cursor *model.ActivityCursor) ([]*model.ActivityEvent, error) {
	if limit <= 0 {
		limit = 20
	}

	var rows *sql.Rows
	var err error

	if cursor != nil {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, user_id, event_type, entity_type, entity_id, class_id, metadata, occurred_at
			FROM activity_events
			WHERE user_id = $1 AND (occurred_at, id) < ($2, $3)
			ORDER BY occurred_at DESC, id DESC
			LIMIT $4
		`, userID, cursor.Timestamp, cursor.ID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, user_id, event_type, entity_type, entity_id, class_id, metadata, occurred_at
			FROM activity_events
			WHERE user_id = $1
			ORDER BY occurred_at DESC, id DESC
			LIMIT $2
		`, userID, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*model.ActivityEvent
	for rows.Next() {
		e := &model.ActivityEvent{}
		if err := rows.Scan(&e.ID, &e.UserID, &e.EventType, &e.EntityType, &e.EntityID, &e.ClassID, &e.Metadata, &e.OccurredAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// EncodeCursor creates an opaque base64 cursor from timestamp + ID.
func EncodeCursor(ts time.Time, id int64) string {
	type cursorData struct {
		Timestamp string `json:"t"`
		ID        int64  `json:"i"`
	}
	data := cursorData{Timestamp: ts.UTC().Format(time.RFC3339Nano), ID: id}
	b, _ := json.Marshal(data)
	return base64.URLEncoding.EncodeToString(b)
}

// DecodeCursor parses an opaque base64 cursor.
func DecodeCursor(encoded string) (*model.ActivityCursor, error) {
	b, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	type cursorData struct {
		Timestamp string `json:"t"`
		ID        int64  `json:"i"`
	}
	var data cursorData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("invalid cursor: %w", err)
	}
	ts, err := time.Parse(time.RFC3339Nano, data.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("invalid cursor timestamp: %w", err)
	}
	return &model.ActivityCursor{Timestamp: ts, ID: data.ID}, nil
}
