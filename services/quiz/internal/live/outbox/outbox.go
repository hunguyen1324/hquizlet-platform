// Package outbox implements the outbox worker that publishes pending events to NATS.
// Dev 2 - [P6-OUTBOX-01]
package outbox

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/events"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/repository"
)

// Publisher is the interface for publishing domain events.
type Publisher interface {
	Publish(ctx context.Context, subject string, event interface{}) error
	IsConnected() bool
}

const (
	batchSize     = 50
	pollInterval  = 2 * time.Second
	maxAttempts   = 10
	backoffBase   = 1 * time.Second
	backoffMax    = 30 * time.Second
)

// Worker polls the outbox and publishes events to NATS.
type Worker struct {
	repo      *repository.DB
	publisher Publisher
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewWorker creates a new outbox worker.
func NewWorker(repo *repository.DB, publisher Publisher) *Worker {
	return &Worker{
		repo:      repo,
		publisher: publisher,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the outbox polling loop.
func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-w.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.processBatch(ctx)
			}
		}
	}()
	log.Printf("[outbox] worker started, poll_interval=%v, batch_size=%d", pollInterval, batchSize)
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
	log.Printf("[outbox] worker stopped")
}

func (w *Worker) processBatch(ctx context.Context) {
	events_batch, err := w.repo.ClaimOutboxBatch(ctx, batchSize)
	if err != nil {
		log.Printf("[outbox] claim batch error: %v", err)
		return
	}
	if len(events_batch) == 0 {
		return
	}

	var published []string
	for _, e := range events_batch {
		if e.Attempts >= maxAttempts {
			log.Printf("[outbox] event %s exceeded max attempts (%d), marking as failed", e.EventID, maxAttempts)
			_ = w.repo.MarkPublishFailed(ctx, e.EventID, "exceeded max attempts")
			continue
		}
		env := &events.EventEnvelope{}
		_ = json.Unmarshal(e.Payload, env)
		env.EventID = e.EventID
		env.OccurredAt = e.OccurredAt
		if err := w.publisher.Publish(ctx, e.Subject, *env); err != nil {
			log.Printf("[outbox] publish failed for %s: %v", e.EventID, err)
			_ = w.repo.MarkPublishFailed(ctx, e.EventID, err.Error())
			continue
		}
		published = append(published, e.EventID)
	}

	if len(published) > 0 {
		if err := w.repo.MarkPublished(ctx, published); err != nil {
			log.Printf("[outbox] mark published error: %v", err)
		}
	}
}
