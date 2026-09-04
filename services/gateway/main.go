package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type serviceHealth struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

type verifiedIdentity struct {
	Authenticated bool   `json:"authenticated"`
	UserID        int64  `json:"userId"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Role          string `json:"role"`
}

func main() {
	port := env("PORT", "8080")
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health("gateway"))
	mux.HandleFunc("GET /healthz/services", servicesHealth)

	authURL := env("AUTH_SERVICE_URL", "http://localhost:8081")
	studyURL := env("STUDY_SERVICE_URL", "http://localhost:8082")
	quizURL := env("QUIZ_SERVICE_URL", "http://localhost:8083")
	classURL := env("CLASS_SERVICE_URL", "http://localhost:8084")
	paymentURL := env("PAYMENT_SERVICE_URL", "http://localhost:8085")
	fileURL := env("FILE_SERVICE_URL", "http://localhost:8086")
	mux.HandleFunc("/v1/auth/", reverseProxy(authURL))
	// /v1/study-sets/{id} goes to study; /v1/study-sets/{id}/quiz/* goes to quiz.
	mux.HandleFunc("/v1/study-sets", authenticatedProxy(authURL, studyURL))
	mux.HandleFunc("/v1/study-sets/", routeStudySets(authURL, studyURL, quizURL))
	mux.HandleFunc("/v1/flashcards/", authenticatedProxy(authURL, studyURL))
	mux.HandleFunc("/v1/folders", authenticatedProxy(authURL, studyURL))
	mux.HandleFunc("/v1/folders/", authenticatedProxy(authURL, studyURL))

	// Phase 6: Live Session Routes [P6-GW-01]
	// Host endpoints -> authenticated proxy
	mux.HandleFunc("POST /v1/live-sessions", authenticatedProxy(authURL, quizURL))
	mux.HandleFunc("GET /v1/live-sessions/{sessionId}", authenticatedProxy(authURL, quizURL))
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/start", authenticatedProxy(authURL, quizURL))
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/questions/current/close", authenticatedProxy(authURL, quizURL))
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/questions/next", authenticatedProxy(authURL, quizURL))
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/end", authenticatedProxy(authURL, quizURL))
	mux.HandleFunc("GET /v1/live-sessions/{sessionId}/leaderboard", authenticatedOrParticipantProxy(authURL, quizURL))

	// Public join endpoint (rate-limited)
	mux.HandleFunc("POST /v1/live-sessions/{code}/join", rateLimitProxy(quizURL))

	// Player endpoints (participant token verified by Quiz service)
	mux.HandleFunc("POST /v1/live-sessions/{sessionId}/answers", participantProxy(quizURL))
	mux.HandleFunc("GET /v1/live-sessions/{sessionId}/player-state", participantProxy(quizURL))

	// SSE endpoint (host or player scope)
	mux.HandleFunc("GET /v1/live-sessions/{sessionId}/events", sseProxy(authURL, quizURL))

	// Phase 7: Class and Activity routes [P7-GW-01]
	mux.HandleFunc("/v1/classes", authenticatedProxy(authURL, classURL))
	mux.HandleFunc("/v1/classes/", authenticatedProxy(authURL, classURL))
	mux.HandleFunc("/v1/activity", authenticatedProxy(authURL, classURL))

	// Phase 9: File upload routes [P9-GW-01]
	mux.HandleFunc("POST /v1/files/presign", authenticatedProxy(authURL, fileURL))
	mux.HandleFunc("/v1/files/", authenticatedProxy(authURL, fileURL))
	mux.HandleFunc("GET /v1/files", authenticatedProxy(authURL, fileURL))

	// Phase 10: TTS & Language routes → Auth service
	mux.HandleFunc("GET /v1/tts", authenticatedProxy(authURL, authURL))
	mux.HandleFunc("GET /v1/languages", authenticatedProxy(authURL, authURL))

	// Phase 10: Import routes → Study service (authenticated)
	// Import routes are matched via /v1/study-sets/ wildcard above, no extra route needed

	// Phase 10: Template download → Study service (no auth required)
	mux.HandleFunc("GET /v1/templates/", reverseProxy(studyURL))

	// Phase 8: Payment, Wallet, and Entitlement routes [P8-GW-01]
	// Webhook: forwarded raw (NO authenticatedProxy), SePay verifies via Apikey header
	mux.HandleFunc("POST /v1/payments/webhooks/sepay", reverseProxy(paymentURL))
	// Internal endpoint for other services (Study service calls this)
	mux.HandleFunc("GET /internal/payment/", reverseProxy(paymentURL))
	// Authenticated payment routes
	mux.HandleFunc("GET /v1/wallet", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("GET /v1/wallet/transactions", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("POST /v1/payments/orders", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("GET /v1/payments/orders/pending", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("DELETE /v1/payments/orders/", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("GET /v1/payments/orders/", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("POST /v1/entitlements/purchase", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("GET /v1/entitlements/check", optionalAuthenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("GET /v1/entitlements", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("PUT /v1/study-sets/", routeStudySetPrice(authURL, studyURL, paymentURL))
	// Admin payment routes
	mux.HandleFunc("GET /v1/admin/payments/orders", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("GET /v1/admin/wallet/transactions", authenticatedProxy(authURL, paymentURL))
	mux.HandleFunc("POST /v1/admin/wallet/credit", authenticatedProxy(authURL, paymentURL))

	log.Printf("[gateway] listening on :%s", port)
	if err := http.ListenAndServe(":"+port, cors(logging(requestID(mux)))); err != nil {
		log.Fatal(err)
	}
}

func routeStudySets(authTarget, studyTarget, quizTarget string) http.HandlerFunc {
	studyHandler := authenticatedProxy(authTarget, studyTarget)
	quizHandler := authenticatedProxy(authTarget, quizTarget)
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/quiz/") {
			quizHandler(w, r)
			return
		}
		studyHandler(w, r)
	}
}

// routeStudySetPrice routes PUT /v1/study-sets/{id}/price to payment service.
func routeStudySetPrice(authTarget, studyTarget, paymentTarget string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/price") && r.Method == http.MethodPut {
			authenticatedProxy(authTarget, paymentTarget)(w, r)
			return
		}
		authenticatedProxy(authTarget, studyTarget)(w, r)
	}
}

func servicesHealth(w http.ResponseWriter, r *http.Request) {
	services := []serviceHealth{
		{Name: "gateway", URL: "http://localhost:" + env("PORT", "8080") + "/healthz"},
		{Name: "auth", URL: env("AUTH_SERVICE_URL", "http://localhost:8081") + "/healthz"},
		{Name: "study", URL: env("STUDY_SERVICE_URL", "http://localhost:8082") + "/healthz"},
		{Name: "quiz", URL: env("QUIZ_SERVICE_URL", "http://localhost:8083") + "/healthz"},
		{Name: "class", URL: env("CLASS_SERVICE_URL", "http://localhost:8084") + "/healthz"},
		{Name: "payment", URL: env("PAYMENT_SERVICE_URL", "http://localhost:8085") + "/healthz"},
		{Name: "file", URL: env("FILE_SERVICE_URL", "http://localhost:8086") + "/healthz"},
	}
	for i := range services {
		if services[i].Name == "gateway" {
			services[i].Status = "ok"
			continue
		}
		services[i].Status = checkHealth(r.Context(), services[i].URL)
	}
	writeJSON(w, http.StatusOK, map[string][]serviceHealth{"services": services})
}

func checkHealth(ctx context.Context, url string) string {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "offline"
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "offline"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "offline"
	}
	return "ok"
}

// authenticatedProxy verifies the bearer token with Auth before forwarding.
// Client-supplied X-User-ID is never trusted.
func authenticatedProxy(authTarget, serviceTarget string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, status, err := verifyIdentity(r.Context(), authTarget, r.Header.Get("Authorization"))
		if err != nil {
			if status == http.StatusUnauthorized {
				writeGatewayError(w, r, status, "UNAUTHORIZED", "authentication required")
				return
			}
			writeGatewayError(w, r, status, "AUTH_UNAVAILABLE", "authentication service unavailable")
			return
		}
		// Strip all client-supplied identity headers [P6-SEC-01]
		r.Header.Del("X-User-ID")
		r.Header.Del("X-User-Role")
		r.Header.Del("X-Participant-ID")
		r.Header.Del("X-Live-Role")
		r.Header.Del("X-Class-Role")
		r.Header.Del("X-Member-ID")
		r.Header.Set("X-User-ID", strconv.FormatInt(identity.UserID, 10))
		r.Header.Set("X-User-Role", identity.Role)
		reverseProxy(serviceTarget)(w, r)
	}
}

// optionalAuthenticatedProxy forwards anonymous requests, but verifies and
// injects identity when a bearer token is present.
func optionalAuthenticatedProxy(authTarget, serviceTarget string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		r.Header.Del("X-User-ID")
		r.Header.Del("X-User-Role")
		r.Header.Del("X-Participant-ID")
		r.Header.Del("X-Live-Role")
		r.Header.Del("X-Class-Role")
		r.Header.Del("X-Member-ID")

		if strings.HasPrefix(auth, "Bearer ") {
			identity, status, err := verifyIdentity(r.Context(), authTarget, auth)
			if err != nil {
				if status == http.StatusUnauthorized {
					writeGatewayError(w, r, status, "UNAUTHORIZED", "authentication required")
					return
				}
				writeGatewayError(w, r, status, "AUTH_UNAVAILABLE", "authentication service unavailable")
				return
			}
			r.Header.Set("X-User-ID", strconv.FormatInt(identity.UserID, 10))
			r.Header.Set("X-User-Role", identity.Role)
		}

		reverseProxy(serviceTarget)(w, r)
	}
}

// participantProxy forwards participant-token requests to Quiz for server-side verification.
// Never injects participant ID from client headers.
func participantProxy(serviceTarget string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Strip spoofed identity headers
		r.Header.Del("X-User-ID")
		r.Header.Del("X-Participant-ID")
		r.Header.Del("X-Live-Role")
		// Forward the Authorization header as-is; Quiz service verifies the token
		reverseProxy(serviceTarget)(w, r)
	}
}

// authenticatedOrParticipantProxy accepts either host auth or participant token.
func authenticatedOrParticipantProxy(authTarget, serviceTarget string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			// Try host auth first
			_, status, err := verifyIdentity(r.Context(), authTarget, auth)
			if err == nil && status == http.StatusOK {
				// Host authenticated
				r.Header.Del("X-User-ID")
				r.Header.Del("X-Participant-ID")
				r.Header.Del("X-Live-Role")
				identity, _, _ := verifyIdentity(r.Context(), authTarget, auth)
				r.Header.Set("X-User-ID", strconv.FormatInt(identity.UserID, 10))
				reverseProxy(serviceTarget)(w, r)
				return
			}
			// Fall through to participant proxy
		}
		participantProxy(serviceTarget)(w, r)
	}
}

// rateLimitProxy is a public proxy with basic rate limiting for join endpoints.
func rateLimitProxy(serviceTarget string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Strip all identity headers from public requests
		r.Header.Del("X-User-ID")
		r.Header.Del("X-Participant-ID")
		r.Header.Del("X-Live-Role")
		r.Header.Del("Authorization")
		// Basic rate limit could be added here
		reverseProxy(serviceTarget)(w, r)
	}
}

// sseProxy handles SSE streaming with proper flush and cancel propagation [P6-GW-02]
func sseProxy(authTarget, serviceTarget string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Determine if this is a host or participant request
		auth := r.Header.Get("Authorization")
		isHost := false
		if strings.HasPrefix(auth, "Bearer ") {
			identity, status, err := verifyIdentity(r.Context(), authTarget, auth)
			if err == nil && status == http.StatusOK {
				isHost = true
				r.Header.Del("X-User-ID")
				r.Header.Del("X-Participant-ID")
				r.Header.Del("X-Live-Role")
				r.Header.Set("X-User-ID", strconv.FormatInt(identity.UserID, 10))
			}
		}
		if !isHost {
			// Participant SSE
			r.Header.Del("X-User-ID")
			r.Header.Del("X-Participant-ID")
			r.Header.Del("X-Live-Role")
		}

		// Make upstream request with context for cancel propagation
		targetURL := strings.TrimRight(serviceTarget, "/") + r.URL.RequestURI()
		req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, nil)
		if err != nil {
			writeGatewayError(w, r, http.StatusBadGateway, "BAD_GATEWAY", "invalid upstream request")
			return
		}
		req.Header = r.Header.Clone()
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Host", r.Host)

		// Don't use default client timeout for SSE
		client := &http.Client{Timeout: 0}
		resp, err := client.Do(req)
		if err != nil {
			writeGatewayError(w, r, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "upstream unavailable")
			return
		}
		defer resp.Body.Close()

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(resp.StatusCode)

		flusher, ok := w.(http.Flusher)

		// Stream response with flush
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = io.WriteString(w, line+"\n")
			// Flush after each line for SSE
			if ok {
				flusher.Flush()
			}
			// Check client disconnect
			if r.Context().Err() != nil {
				return
			}
		}
	}
}

func verifyIdentity(ctx context.Context, authTarget, authorization string) (verifiedIdentity, int, error) {
	var identity verifiedIdentity
	token := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	if !strings.HasPrefix(authorization, "Bearer ") || token == "" {
		return identity, http.StatusUnauthorized, context.Canceled
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(authTarget, "/")+"/internal/auth/verify", nil)
	if err != nil {
		return identity, http.StatusBadGateway, err
	}
	req.Header.Set("Authorization", authorization)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return identity, http.StatusBadGateway, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return identity, http.StatusUnauthorized, context.Canceled
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return identity, http.StatusBadGateway, context.Canceled
	}
	if err := json.NewDecoder(resp.Body).Decode(&identity); err != nil {
		return identity, http.StatusBadGateway, err
	}
	if !identity.Authenticated || identity.UserID <= 0 {
		return identity, http.StatusUnauthorized, context.Canceled
	}
	return identity, http.StatusOK, nil
}

func cors(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{
		"http://localhost:5173": true, "http://127.0.0.1:5173": true,
		"http://localhost:3000": true, "http://127.0.0.1:3000": true,
		"http://web:5173": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] || isAllowedDevOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Last-Event-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isAllowedDevOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if u.Port() != "5173" && u.Port() != "3000" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "web" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsPrivate()
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = time.Now().UTC().Format("20060102150405.000000000")
		}
		w.Header().Set("X-Request-ID", id)
		r.Header.Set("X-Request-ID", id)
		next.ServeHTTP(w, r)
	})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Printf("[gateway] %s %s -> %d (%s)", r.Method, r.URL.Path, rw.status, time.Since(start).Round(time.Millisecond))
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func health(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": service, "status": "ok"})
	}
}

func reverseProxy(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		targetURL := strings.TrimRight(target, "/") + r.URL.RequestURI()
		req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
		if err != nil {
			writeGatewayError(w, r, http.StatusBadGateway, "BAD_GATEWAY", "invalid upstream request")
			return
		}
		req.Header = r.Header.Clone()
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Host", r.Host)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeGatewayError(w, r, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "upstream unavailable")
			return
		}
		defer resp.Body.Close()
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

type errorEnvelope struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RequestID string         `json:"requestId"`
	Details   map[string]any `json:"details"`
}

func writeGatewayError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Code: code, Message: message, RequestID: r.Header.Get("X-Request-ID"), Details: map[string]any{}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
