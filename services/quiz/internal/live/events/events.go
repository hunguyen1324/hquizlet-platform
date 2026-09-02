// Package events provides NATS JetStream publishing for live quiz domain events.
// Dev 2 - [P6-NATS-01]
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// EventEnvelope is the versioned event envelope.
type EventEnvelope struct {
	EventID      string      `json:"eventId"`
	EventType    string      `json:"eventType"`
	EventVersion int         `json:"eventVersion"`
	AggregateID  string      `json:"aggregateId"`
	OccurredAt   time.Time   `json:"occurredAt"`
	RequestID    string      `json:"requestId"`
	Data         interface{} `json:"data"`
}

// Subjects for live quiz events.
const (
	SubjectSessionCreated     = "hquizlet.live.session.created.v1"
	SubjectParticipantJoined  = "hquizlet.live.participant.joined.v1"
	SubjectSessionStarted     = "hquizlet.live.session.started.v1"
	SubjectQuestionOpened     = "hquizlet.live.question.opened.v1"
	SubjectAnswerSubmitted    = "hquizlet.live.answer.submitted.v1"
	SubjectQuestionClosed     = "hquizlet.live.question.closed.v1"
	SubjectSessionEnded       = "hquizlet.live.session.ended.v1"
)

// Publisher publishes events to NATS JetStream.
type Publisher struct {
	nc *nats.Conn
}

// NewPublisher creates a new event publisher.
func NewPublisher(nc *nats.Conn) *Publisher {
	return &Publisher{nc: nc}
}

// Publish sends a domain event to NATS JetStream.
func (p *Publisher) Publish(ctx context.Context, subject string, event EventEnvelope) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	if p.nc == nil {
		log.Printf("[events] NATS not connected, dropping event %s", event.EventID)
		return nil
	}
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  make(nats.Header),
	}
	msg.Header.Set("Nats-Msg-Id", event.EventID)
	// Try JetStream first, fall back to core NATS
	if js, jsErr := p.nc.JetStream(); jsErr == nil {
		if _, err := js.PublishMsg(msg); err != nil {
			// Fall back to core NATS publish
			if pubErr := p.nc.PublishMsg(msg); pubErr != nil {
				return fmt.Errorf("publish event: %w", pubErr)
			}
		}
	} else {
		if pubErr := p.nc.PublishMsg(msg); pubErr != nil {
			return fmt.Errorf("publish event: %w", pubErr)
		}
	}
	return nil
}

// IsConnected returns whether the publisher is connected.
func (p *Publisher) IsConnected() bool {
	return p.nc != nil && p.nc.IsConnected()
}
