// Package http also contains the auto-close scheduler.
// Dev 3 - [P6-TIMER-01]
package http

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/realtime"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/service"
)

// Scheduler manages auto-close timers for open questions.
type Scheduler struct {
	svc         *service.Service
	broadcaster *realtime.Broadcaster
	timers      map[int64]*time.Timer
	mu          sync.Mutex
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewScheduler creates a new auto-close scheduler.
func NewScheduler(svc *service.Service, broadcaster *realtime.Broadcaster) *Scheduler {
	return &Scheduler{
		svc:         svc,
		broadcaster: broadcaster,
		timers:      make(map[int64]*time.Timer),
		stopCh:      make(chan struct{}),
	}
}

// ScheduleAutoClose sets up a timer to auto-close a question after its duration.
func (s *Scheduler) ScheduleAutoClose(sessionID int64, durationMs int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel existing timer
	if t, ok := s.timers[sessionID]; ok {
		t.Stop()
	}

	s.timers[sessionID] = time.AfterFunc(time.Duration(durationMs)*time.Millisecond, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		defer s.CancelTimer(sessionID)
		log.Printf("[scheduler] auto-closing question for session %d", sessionID)
		session, err := s.svc.GetSession(ctx, sessionID)
		if err != nil {
			return
		}
		// Auto-close is idempotent
		if session.Status != "QUESTION_OPEN" {
			return
		}
		_, err = s.svc.CloseQuestion(ctx, sessionID, session.HostUserID, "auto-close")
		if err != nil {
			log.Printf("[scheduler] auto-close error for session %d: %v", sessionID, err)
			return
		}
		s.broadcaster.BroadcastEvent(sessionID, "auto-close-"+time.Now().Format("20060102150405"), "question.closed", map[string]interface{}{
			"sessionId": sessionID,
			"reason":    "timeout",
		})
		if entries, lbErr := s.svc.GetLeaderboard(ctx, sessionID); lbErr == nil {
			s.broadcaster.BroadcastEvent(sessionID, "auto-lb-"+time.Now().Format("20060102150405"), "leaderboard.updated", map[string]interface{}{
				"rankings": entries,
			})
		}
	})
}

// CancelTimer cancels the auto-close timer for a session.
func (s *Scheduler) CancelTimer(sessionID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.timers[sessionID]; ok {
		t.Stop()
		delete(s.timers, sessionID)
	}
}

// Start begins the scheduler.
func (s *Scheduler) Start(ctx context.Context) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-s.stopCh
	}()
	log.Printf("[scheduler] started")
}

// Stop gracefully shuts down the scheduler.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	s.mu.Lock()
	for id, t := range s.timers {
		t.Stop()
		delete(s.timers, id)
	}
	s.mu.Unlock()
	s.wg.Wait()
	log.Printf("[scheduler] stopped")
}
