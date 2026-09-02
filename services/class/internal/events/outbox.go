package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"
)

// outboxRow represents one row from the outbox table.
type outboxRow struct {
	EventID      string
	AggregateID  int64
	Subject      string
	EventVersion int
	Payload      []byte
	OccurredAt   time.Time
	Attempts     int
}

// OutboxWorker polls the class_event_outbox table and publishes events to NATS.
type OutboxWorker struct {
	db        *sql.DB
	publisher *Publisher
	interval  time.Duration
}

// NewOutboxWorker creates a new outbox worker.
func NewOutboxWorker(db *sql.DB, publisher *Publisher) *OutboxWorker {
	return &OutboxWorker{
		db:        db,
		publisher: publisher,
		interval:  5 * time.Second,
	}
}

// Start begins the outbox polling loop. It runs until the context is cancelled.
func (w *OutboxWorker) Start(ctx context.Context) {
	if w.publisher == nil {
		log.Printf("[class-outbox] warning: no publisher, outbox worker disabled")
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("[class-outbox] starting outbox worker (interval=%s)", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Printf("[class-outbox] stopping outbox worker")
			return
		case <-ticker.C:
			if err := w.pollOnce(ctx); err != nil {
				log.Printf("[class-outbox] poll error: %v", err)
			}
		}
	}
}

func (w *OutboxWorker) pollOnce(ctx context.Context) error {
	rows, err := w.db.QueryContext(ctx, `
		SELECT event_id, aggregate_id, subject, event_version, payload, occurred_at, attempts
		FROM class_event_outbox
		WHERE published_at IS NULL
		ORDER BY occurred_at ASC
		LIMIT 50
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var events []outboxRow
	for rows.Next() {
		var row outboxRow
		if err := rows.Scan(&row.EventID, &row.AggregateID, &row.Subject, &row.EventVersion, &row.Payload, &row.OccurredAt, &row.Attempts); err != nil {
			return err
		}
		events = append(events, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, event := range events {
		if err := w.publishEvent(ctx, event); err != nil {
			log.Printf("[class-outbox] failed to publish event %s: %v", event.EventID, err)
			_, _ = w.db.ExecContext(ctx, `
				UPDATE class_event_outbox
				SET attempts = attempts + 1, last_error = $1
				WHERE event_id = $2
			`, err.Error(), event.EventID)
			continue
		}

		_, err := w.db.ExecContext(ctx, `
			UPDATE class_event_outbox
			SET published_at = NOW()
			WHERE event_id = $1
		`, event.EventID)
		if err != nil {
			log.Printf("[class-outbox] failed to mark event %s as published: %v", event.EventID, err)
		}
	}

	return nil
}

func (w *OutboxWorker) publishEvent(ctx context.Context, row outboxRow) error {
	var data json.RawMessage
	if err := json.Unmarshal(row.Payload, &data); err != nil {
		data = row.Payload
	}

	env := EventEnvelope{
		EventID:      row.EventID,
		EventType:    "",
		EventVersion: row.EventVersion,
		AggregateID:  "",
		OccurredAt:   row.OccurredAt.UTC().Format(time.RFC3339),
		Data:         data,
	}

	return w.publisher.Publish(row.Subject, env)
}
