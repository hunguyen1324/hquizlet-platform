package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/engine"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/studyclient"

	livehttp "github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/http"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/model"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/outbox"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/realtime"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/redisstore"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/quiz/internal/live/service"
	quizmigrations "github.com/hunguyen1324/hquizlet-platform/services/quiz/migrations"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

type metrics struct {
	Requests    map[string]int
	Errors      map[string]int
	LatencyMS   map[string]int64
	LatencyHits map[string]int
	mu          sync.Mutex
}

func newMetrics() *metrics {
	return &metrics{
		Requests:    make(map[string]int),
		Errors:      make(map[string]int),
		LatencyMS:   make(map[string]int64),
		LatencyHits: make(map[string]int),
	}
}

func (m *metrics) observe(endpoint, mode string, status int, elapsed time.Duration) {
	if mode == "" {
		mode = "unknown"
	}
	key := endpoint + ":" + mode
	m.mu.Lock()
	m.Requests[key]++
	m.LatencyMS[key] += elapsed.Milliseconds()
	m.LatencyHits[key]++
	if status >= 400 {
		m.Errors[key]++
	}
	m.mu.Unlock()
}

func (m *metrics) writePrometheus(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for key, value := range m.Requests {
		endpoint, mode := splitMetricKey(key)
		_, _ = fmt.Fprintf(w, "quiz_requests_total{endpoint=%q,mode=%q} %d\n", endpoint, mode, value)
	}
	for key, value := range m.Errors {
		endpoint, mode := splitMetricKey(key)
		_, _ = fmt.Fprintf(w, "quiz_errors_total{endpoint=%q,mode=%q} %d\n", endpoint, mode, value)
	}
	for key, value := range m.LatencyMS {
		endpoint, mode := splitMetricKey(key)
		_, _ = fmt.Fprintf(w, "quiz_latency_ms_sum{endpoint=%q,mode=%q} %d\n", endpoint, mode, value)
		_, _ = fmt.Fprintf(w, "quiz_latency_ms_count{endpoint=%q,mode=%q} %d\n", endpoint, mode, m.LatencyHits[key])
	}
}

func splitMetricKey(key string) (string, string) {
	parts := strings.SplitN(key, ":", 2)
	if len(parts) != 2 {
		return key, "unknown"
	}
	return parts[0], parts[1]
}

type server struct {
	study *studyclient.Client
	met   *metrics
}

const (
	dependencyWaitTimeout = 60 * time.Second
	dependencyRetryDelay  = 2 * time.Second
)

type generateRequest struct {
	Mode    string         `json:"mode"`
	Seed    uint64         `json:"seed"`
	Limit   int            `json:"limit"`
	Options map[string]any `json:"options,omitempty"`
}

type evaluateRequest struct {
	Mode    string          `json:"mode"`
	Seed    uint64          `json:"seed"`
	Limit   int             `json:"limit"`
	Answers []engine.Answer `json:"answers"`
}

type generateResponse struct {
	Mode            string        `json:"mode"`
	Seed            uint64        `json:"seed"`
	Items           []engine.Item `json:"items"`
	ContractVersion string        `json:"contractVersion"`
}

type evaluateResponse struct {
	Mode            string              `json:"mode"`
	Seed            uint64              `json:"seed"`
	Score           int                 `json:"score"`
	Total           int                 `json:"total"`
	CardResults     []engine.CardResult `json:"cardResults"`
	ContractVersion string              `json:"contractVersion"`
}

type errorEnvelope struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details,omitempty"`
}

func main() {
	port := env("PORT", "8083")
	ctx := context.Background()

	s := &server{
		study: studyclient.New(env("STUDY_SERVICE_URL", "http://localhost:8082")),
		met:   newMetrics(),
	}

	// --- Phase 6: Live Quiz Dependencies ---

	// PostgreSQL
	var repoDB *repository.DB
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		db, err := sql.Open("postgres", dbURL)
		if err != nil {
			log.Printf("[quiz] warning: cannot open database: %v", err)
		} else {
			db.SetMaxOpenConns(25)
			db.SetMaxIdleConns(5)
			if err := waitForDependency(ctx, "database", func(ctx context.Context) error {
				return db.PingContext(ctx)
			}); err != nil {
				log.Printf("[quiz] warning: database ping failed: %v", err)
				_ = db.Close()
			} else {
				if err := quizmigrations.Run(db); err != nil {
					log.Fatalf("[quiz] migration failed: %v", err)
				}
				repoDB = repository.New(db)
				log.Printf("[quiz] PostgreSQL connected")
			}
		}
	}

	// Redis
	var redisStore *redisstore.Store
	if redisURL := os.Getenv("REDIS_URL"); redisURL != "" {
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			log.Printf("[quiz] warning: invalid REDIS_URL: %v", err)
		} else {
			rdb := redis.NewClient(opts)
			if err := waitForDependency(ctx, "redis", func(ctx context.Context) error {
				return rdb.Ping(ctx).Err()
			}); err != nil {
				log.Printf("[quiz] warning: redis ping failed: %v", err)
				_ = rdb.Close()
			} else {
				redisStore = redisstore.New(rdb)
				log.Printf("[quiz] Redis connected")
			}
		}
	}

	// NATS
	var natsConn *nats.Conn
	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		nc, err := nats.Connect(natsURL,
			nats.Name("quiz-service"),
			nats.RetryOnFailedConnect(true),
			nats.MaxReconnects(-1),
			nats.ReconnectWait(2*time.Second),
		)
		if err != nil {
			log.Printf("[quiz] warning: NATS connect failed: %v", err)
		} else {
			natsConn = nc
			// Ensure the durable stream exists before the outbox starts publishing.
			if js, err := nc.JetStream(); err == nil {
				if _, err := js.StreamInfo("HQUIZLET_LIVE"); err != nil {
					if _, addErr := js.AddStream(&nats.StreamConfig{
						Name: "HQUIZLET_LIVE", Subjects: []string{"hquizlet.live.>"},
						Storage: nats.FileStorage,
					}); addErr != nil {
						log.Printf("[quiz] warning: create JetStream stream failed: %v", addErr)
					}
				}
				log.Printf("[quiz] NATS JetStream available")
			}
			log.Printf("[quiz] NATS connected: %s", natsURL)
		}
	}

	// Live service + handlers
	var liveHandlers *livehttp.Handlers
	var broadcaster *realtime.Broadcaster
	var scheduler *livehttp.Scheduler
	var outboxWorker *outbox.Worker

	if repoDB != nil && redisStore != nil {
		pub := newEventPublisherStub(natsConn)
		svc := service.New(repoDB, redisStore, s.study, pub)
		broadcaster = realtime.NewBroadcaster()
		liveHandlers = livehttp.NewHandlers(svc, broadcaster)
		scheduler = livehttp.NewScheduler(svc, broadcaster)
		liveHandlers.SetScheduler(scheduler)

		// Recover non-terminal sessions and restore auto-close timers.
		recovered, err := svc.RecoverSessions(ctx)
		if err != nil {
			log.Printf("[quiz] session recovery error: %v", err)
		} else {
			for _, session := range recovered {
				if session.Status != model.StatusQuestionOpen {
					continue
				}
				remaining := time.Until(session.UpdatedAt.Add(time.Duration(session.QuestionDurationMs) * time.Millisecond))
				if remaining < time.Millisecond {
					remaining = time.Millisecond
				}
				scheduler.ScheduleAutoClose(session.ID, int(remaining.Milliseconds()))
			}
		}

		// Start outbox worker
		outboxWorker = outbox.NewWorker(repoDB, pub)
		_ = outboxWorker
		outboxWorker.Start(ctx)
		scheduler.Start(ctx)
		log.Printf("[quiz] Live Quiz subsystem initialized")
	} else {
		log.Printf("[quiz] Live Quiz disabled: missing database or redis configuration")
	}

	// --- HTTP Router ---
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "quiz", "status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ready := repoDB != nil && redisStore != nil && natsConn != nil && natsConn.IsConnected()
		status := "ok"
		httpStatus := http.StatusOK
		if !ready {
			status = "degraded"
			httpStatus = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(map[string]string{"service": "quiz", "status": status})
	})
	mux.HandleFunc("GET /metrics", s.met.writePrometheus)

	// Existing deterministic quiz endpoints
	mux.HandleFunc("POST /v1/study-sets/{id}/quiz/generate", s.generate)
	mux.HandleFunc("POST /v1/study-sets/{id}/quiz/evaluate", s.evaluate)

	// Phase 6: Live Quiz endpoints
	if liveHandlers != nil {
		liveHandlers.RegisterRoutes(mux)
	}

	log.Printf("quiz service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

// eventPublisherStub wraps the event publisher for the service interface.
type eventPublisherStub struct {
	nc *nats.Conn
}

func newEventPublisherStub(nc *nats.Conn) *eventPublisherStub {
	return &eventPublisherStub{nc: nc}
}

func (p *eventPublisherStub) Publish(ctx context.Context, subject string, event interface{}) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if p.nc == nil || !p.nc.IsConnected() {
		return fmt.Errorf("NATS unavailable")
	}
	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	js, err := p.nc.JetStream()
	if err != nil {
		return err
	}
	msg.Header.Set(nats.MsgIdHdr, eventIDFromPayload(data))
	_, err = js.PublishMsg(msg)
	return err
}

func eventIDFromPayload(data []byte) string {
	var envelope struct {
		EventID string `json:"eventId"`
	}
	_ = json.Unmarshal(data, &envelope)
	return envelope.EventID
}

func waitForDependency(ctx context.Context, name string, ping func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, dependencyWaitTimeout)
	defer cancel()

	var lastErr error
	for {
		if err := ping(ctx); err != nil {
			lastErr = err
			log.Printf("[quiz] waiting for %s: %v", name, err)
		} else {
			return nil
		}

		timer := time.NewTimer(dependencyRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (p *eventPublisherStub) IsConnected() bool {
	return p.nc != nil && p.nc.IsConnected()
}

func (s *server) generate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := r.Header.Get("X-Request-ID")
	userID := r.Header.Get("X-User-ID")
	set, ok := s.loadSet(w, r)
	if !ok {
		return
	}
	var req generateRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Limit == 0 {
		req.Limit = 100
	}
	status := http.StatusOK
	defer func() {
		s.met.observe("generate", req.Mode, status, time.Since(start))
		log.Printf("[generate] request_id=%s uid=%s set=%d mode=%s seed=%d status=%d duration_ms=%d", reqID, userID, set.ID, req.Mode, req.Seed, status, time.Since(start).Milliseconds())
	}()
	items, err := engine.Generate(set.Flashcards, req.Mode, req.Seed, req.Limit)
	if err != nil {
		status = http.StatusUnprocessableEntity
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "invalid mode, seed, or limit")
		return
	}
	writeJSON(w, http.StatusOK, generateResponse{Mode: req.Mode, Seed: req.Seed, Items: items, ContractVersion: engine.ContractVersion})
}

func (s *server) evaluate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	reqID := r.Header.Get("X-Request-ID")
	userID := r.Header.Get("X-User-ID")
	set, ok := s.loadSet(w, r)
	if !ok {
		return
	}
	var req evaluateRequest
	if !decode(w, r, &req) {
		return
	}
	if req.Limit == 0 {
		req.Limit = len(req.Answers)
	}
	status := http.StatusOK
	defer func() {
		s.met.observe("evaluate", req.Mode, status, time.Since(start))
		log.Printf("[evaluate] request_id=%s uid=%s set=%d mode=%s seed=%d limit=%d status=%d duration_ms=%d", reqID, userID, set.ID, req.Mode, req.Seed, req.Limit, status, time.Since(start).Milliseconds())
	}()
	results, err := engine.Evaluate(set.Flashcards, req.Mode, req.Seed, req.Limit, req.Answers)
	if err != nil {
		status = http.StatusUnprocessableEntity
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "answers do not match this study set or quiz mode")
		return
	}
	score := 0
	for _, result := range results {
		if result.Correct {
			score++
		}
	}
	writeJSON(w, http.StatusOK, evaluateResponse{Mode: req.Mode, Seed: req.Seed, Score: score, Total: len(results), CardResults: results, ContractVersion: engine.ContractVersion})
}

func (s *server) loadSet(w http.ResponseWriter, r *http.Request) (*studyclient.StudySetWithCards, bool) {
	setID, err1 := strconv.ParseInt(r.PathValue("id"), 10, 64)
	userID, err2 := strconv.ParseInt(r.Header.Get("X-User-ID"), 10, 64)
	if err1 != nil || setID < 1 {
		writeError(w, r, http.StatusBadRequest, "INVALID_STUDY_SET_ID", "invalid study set id")
		return nil, false
	}
	if err2 != nil || userID < 1 {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return nil, false
	}
	set, err := s.study.GetFlashcards(r.Context(), setID, userID)
	if err == nil {
		return set, true
	}
	if errors.Is(err, studyclient.ErrForbidden) {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "study set is not owned by caller")
	} else if errors.Is(err, studyclient.ErrNotFound) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "study set not found")
	} else {
		writeError(w, r, http.StatusBadGateway, "STUDY_UNAVAILABLE", "study service unavailable")
	}
	return nil, false
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Code: code, Message: message, RequestID: r.Header.Get("X-Request-ID"), Details: map[string]any{}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
