// Package events provides NATS JetStream event publishing for the class service.
package events

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// EventEnvelope is the standard event envelope for NATS publishing.
type EventEnvelope struct {
	EventID      string          `json:"eventId"`
	EventType    string          `json:"eventType"`
	EventVersion int             `json:"eventVersion"`
	AggregateID  string          `json:"aggregateId"`
	OccurredAt   string          `json:"occurredAt"`
	RequestID    string          `json:"requestId"`
	Data         json.RawMessage `json:"data"`
}

// Publisher publishes class events to NATS JetStream.
type Publisher struct {
	conn   *nats.Conn
	js     nats.JetStreamContext
	stream string
}

// NewPublisher creates a new NATS publisher. Returns nil if NATS is unavailable.
func NewPublisher(natsURL, stream string) *Publisher {
	if natsURL == "" {
		return nil
	}

	conn, err := nats.Connect(natsURL,
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(conn *nats.Conn, err error) {
			log.Printf("[class-events] nats disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(conn *nats.Conn) {
			log.Printf("[class-events] nats reconnected to %s", conn.ConnectedUrl())
		}),
	)
	if err != nil {
		log.Printf("[class-events] warning: nats unavailable, events will not be published: %v", err)
		return nil
	}

	js, err := conn.JetStream()
	if err != nil {
		log.Printf("[class-events] warning: jetstream unavailable: %v", err)
		conn.Close()
		return nil
	}

	// Ensure stream exists
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     stream,
		Subjects: []string{stream + ".>"},
	})
	if err != nil {
		log.Printf("[class-events] warning: could not create stream %s: %v", stream, err)
	}

	return &Publisher{conn: conn, js: js, stream: stream}
}

// Publish publishes an event to NATS JetStream with Msg-Id deduplication.
func (p *Publisher) Publish(subject string, event EventEnvelope) error {
	if p == nil || p.js == nil {
		return nil // no-op if NATS unavailable
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	_, err = p.js.Publish(subject, data, nats.MsgId(event.EventID))
	if err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}
	return nil
}

// Close closes the NATS connection.
func (p *Publisher) Close() {
	if p != nil && p.conn != nil {
		p.conn.Close()
	}
}
