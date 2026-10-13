package repository

import (
	"context"
	"database/sql"
	"github.com/nats-io/nats.go"
	"log"
	"time"
)

// RunCompletionOutbox retries until JetStream acknowledges the persisted completion event.
func RunCompletionOutbox(ctx context.Context, db *sql.DB, url string) {
	if url == "" {
		return
	}
	for ctx.Err() == nil {
		conn, err := nats.Connect(url, nats.Timeout(3*time.Second))
		if err != nil {
			log.Printf("[study] completion NATS unavailable")
			time.Sleep(5 * time.Second)
			continue
		}
		js, err := conn.JetStream()
		if err != nil {
			conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		if _, err = js.AddStream(&nats.StreamConfig{Name: "STUDY_COMPLETIONS", Subjects: []string{"study.completed"}}); err != nil {
			if _, err = js.StreamInfo("STUDY_COMPLETIONS"); err != nil {
				conn.Close()
				time.Sleep(5 * time.Second)
				continue
			}
		}
		for ctx.Err() == nil && conn.IsConnected() {
			rows, err := db.QueryContext(ctx, `SELECT event_id,payload FROM study_event_outbox WHERE published_at IS NULL ORDER BY created_at LIMIT 50`)
			if err == nil {
				type event struct {
					id   string
					body []byte
				}
				events := []event{}
				for rows.Next() {
					var e event
					if rows.Scan(&e.id, &e.body) == nil {
						events = append(events, e)
					}
				}
				rows.Close()
				for _, e := range events {
					if _, err = js.Publish("study.completed", e.body, nats.MsgId(e.id)); err != nil {
						break
					}
					if _, err = db.ExecContext(ctx, `UPDATE study_event_outbox SET published_at=now() WHERE event_id=$1`, e.id); err != nil {
						log.Printf("[study] outbox acknowledgement persistence failed: %v", err)
					}
				}
			}
			select {
			case <-ctx.Done():
				conn.Close()
				return
			case <-time.After(2 * time.Second):
			}
		}
		conn.Close()
	}
}
