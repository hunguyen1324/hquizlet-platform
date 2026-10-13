package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/nats-io/nats.go"
	"log"
	"time"
)

// RunCompletionConsumer maps a learner/set event to their assigned classes, never arbitrary caller class IDs.
func RunCompletionConsumer(ctx context.Context, db *sql.DB, url string) {
	if url == "" {
		return
	}
	for ctx.Err() == nil {
		conn, err := nats.Connect(url, nats.Timeout(3*time.Second))
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		js, err := conn.JetStream()
		if err != nil {
			conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		_, err = js.AddStream(&nats.StreamConfig{Name: "STUDY_COMPLETIONS", Subjects: []string{"study.completed"}})
		if err != nil {
			if _, err = js.StreamInfo("STUDY_COMPLETIONS"); err != nil {
				conn.Close()
				time.Sleep(5 * time.Second)
				continue
			}
		}
		sub, err := js.PullSubscribe("study.completed", "class-study-completions", nats.BindStream("STUDY_COMPLETIONS"), nats.ManualAck())
		if err != nil {
			conn.Close()
			time.Sleep(5 * time.Second)
			continue
		}
		for ctx.Err() == nil && conn.IsConnected() {
			messages, err := sub.Fetch(20, nats.MaxWait(2*time.Second))
			if err != nil {
				continue
			}
			for _, msg := range messages {
				var e struct {
					EventID    string    `json:"eventId"`
					EventType  string    `json:"eventType"`
					UserID     int64     `json:"userId"`
					StudySetID int64     `json:"studySetId"`
					OccurredAt time.Time `json:"occurredAt"`
				}
				if json.Unmarshal(msg.Data, &e) != nil || e.EventID == "" || e.UserID < 1 || e.StudySetID < 1 {
					msg.Term()
					continue
				}
				_, err = db.ExecContext(ctx, `INSERT INTO activity_events(user_id,event_type,entity_type,entity_id,class_id,metadata,occurred_at,source_event_id)
    SELECT $1,$2,'study-set',$3,c.class_id,$4::jsonb,$5,$6 FROM class_study_sets c JOIN class_members m ON m.class_id=c.class_id AND m.user_id=$1 WHERE c.study_set_id=$3 ON CONFLICT(source_event_id,class_id) DO NOTHING`, e.UserID, e.EventType, e.StudySetID, msg.Data, e.OccurredAt, e.EventID)
				if err != nil {
					log.Printf("[class] completion persistence failed: %v", err)
					msg.Nak()
				} else {
					msg.Ack()
				}
			}
		}
		sub.Unsubscribe()
		conn.Close()
	}
}
