// Package realtime implements the SSE broadcaster for live quiz events.
// Dev 3 - [P6-SSE-01]
package realtime

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/events"
)

const (
	heartbeatInterval = 15 * time.Second
	flushTimeout      = 10 * time.Millisecond
)

// Client represents a connected SSE client.
type Client struct {
	ID            string
	SessionID     int64
	Role          string // "host" or "player"
	UserID        int64  // for host
	ParticipantID string // for player
	LastEventID   string
	Ch            chan []byte
	Done          chan struct{}
}

// IsHost returns true if this is a host client.
func (c *Client) IsHost() bool { return c.Role == "host" }

// Broadcaster manages SSE client connections per session.
type Broadcaster struct {
	mu      sync.RWMutex
	clients map[string]*Client // clientID -> client
}

// NewBroadcaster creates a new broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients: make(map[string]*Client),
	}
}

// Register adds a new SSE client.
func (b *Broadcaster) Register(client *Client) {
	b.mu.Lock()
	b.clients[client.ID] = client
	b.mu.Unlock()
	log.Printf("[sse] client registered: id=%s session=%d role=%s", client.ID, client.SessionID, client.Role)
}

// Unregister removes an SSE client.
func (b *Broadcaster) Unregister(clientID string) {
	b.mu.Lock()
	if c, ok := b.clients[clientID]; ok {
		close(c.Done)
		delete(b.clients, clientID)
		log.Printf("[sse] client unregistered: id=%s", clientID)
	}
	b.mu.Unlock()
}

// BroadcastEvent sends an event to all clients in a session (scoped).
func (b *Broadcaster) BroadcastEvent(sessionID int64, eventID string, eventName string, data interface{}) {
	payload, err := json.Marshal(map[string]interface{}{
		"eventId":    eventID,
		"sessionId":  sessionID,
		"event":      eventName,
		"data":       data,
		"serverTime": time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		log.Printf("[sse] marshal error: %v", err)
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, c := range b.clients {
		if c.SessionID != sessionID {
			continue
		}
		// Player clients should not receive correct answers before reveal
		// This filtering is done at the event level
		select {
		case c.Ch <- payload:
		default:
			// Backpressure: drop if client is too slow
			log.Printf("[sse] dropping event for slow client %s", c.ID)
		}
	}
}

// BroadcastToHost sends an event only to host clients of a session.
func (b *Broadcaster) BroadcastToHost(sessionID int64, eventID string, eventName string, data interface{}) {
	payload, _ := json.Marshal(map[string]interface{}{
		"eventId":    eventID,
		"sessionId":  sessionID,
		"event":      eventName,
		"data":       data,
		"serverTime": time.Now().UTC().Format(time.RFC3339Nano),
	})
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, c := range b.clients {
		if c.SessionID == sessionID && c.IsHost() {
			select {
			case c.Ch <- payload:
			default:
			}
		}
	}
}

// BroadcastToPlayer sends an event only to a specific player.
func (b *Broadcaster) BroadcastToPlayer(sessionID int64, participantID string, eventID string, eventName string, data interface{}) {
	payload, _ := json.Marshal(map[string]interface{}{
		"eventId":    eventID,
		"sessionId":  sessionID,
		"event":      eventName,
		"data":       data,
		"serverTime": time.Now().UTC().Format(time.RFC3339Nano),
	})
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, c := range b.clients {
		if c.SessionID == sessionID && c.ParticipantID == participantID {
			select {
			case c.Ch <- payload:
			default:
			}
		}
	}
}

// ClientCount returns the number of connected clients for a session.
func (b *Broadcaster) ClientCount(sessionID int64) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	count := 0
	for _, c := range b.clients {
		if c.SessionID == sessionID {
			count++
		}
	}
	return count
}

// HandleSSE sets up the SSE response and streams events to the client.
func HandleSSE(w http.ResponseWriter, r *http.Request, client *Client, b *Broadcaster, replayEvents []events.EventEnvelope) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	b.Register(client)
	defer b.Unregister(client.ID)

	ctx := r.Context()

	// Send replay events first
	for _, evt := range replayEvents {
		data, _ := json.Marshal(map[string]interface{}{
			"eventId":    evt.EventID,
			"sessionId":  client.SessionID,
			"event":      evt.EventType,
			"data":       evt.Data,
			"serverTime": evt.OccurredAt.Format(time.RFC3339Nano),
		})
		fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", evt.EventID, evt.EventType, string(data))
		flusher.Flush()
	}

	// Heartbeat ticker
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-client.Done:
			return
		case data, ok := <-client.Ch:
			if !ok {
				return
			}
			// Parse to extract event ID
			var parsed map[string]interface{}
			if err := json.Unmarshal(data, &parsed); err == nil {
				if eid, ok := parsed["eventId"].(string); ok {
					eventName, _ := parsed["event"].(string)
					if eventName == "" {
						eventName = "message"
					}
					fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", eid, eventName, string(data))
				} else {
					fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
				}
			} else {
				fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(data))
			}
			flusher.Flush()
		case <-heartbeat.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: {\"serverTime\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339Nano))
			flusher.Flush()
		}
	}
}

// ClientID generates a unique client ID.
func ClientID() string {
	return fmt.Sprintf("sse-%d", time.Now().UnixNano())
}
