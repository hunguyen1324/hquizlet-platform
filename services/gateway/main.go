package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type serviceHealth struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

func main() {
	port := env("PORT", "8080")

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /healthz", health("gateway"))
	mux.HandleFunc("GET /healthz/services", servicesHealth)

	// Auth service – tất cả route /v1/auth/*
	mux.HandleFunc("/v1/auth/", reverseProxy(env("AUTH_SERVICE_URL", "http://localhost:8081")))

	// Study service – study-sets và flashcards
	mux.HandleFunc("/v1/study-sets", reverseProxy(env("STUDY_SERVICE_URL", "http://localhost:8082")))
	mux.HandleFunc("/v1/study-sets/", reverseProxy(env("STUDY_SERVICE_URL", "http://localhost:8082")))
	mux.HandleFunc("/v1/flashcards/", reverseProxy(env("STUDY_SERVICE_URL", "http://localhost:8082")))

	// Quiz service – live sessions
	mux.HandleFunc("/v1/live-sessions", reverseProxy(env("QUIZ_SERVICE_URL", "http://localhost:8083")))
	mux.HandleFunc("/v1/live-sessions/", reverseProxy(env("QUIZ_SERVICE_URL", "http://localhost:8083")))

	log.Printf("[gateway] listening on :%s", port)
	if err := http.ListenAndServe(":"+port, cors(logging(requestID(mux)))); err != nil {
		log.Fatal(err)
	}
}

func servicesHealth(w http.ResponseWriter, r *http.Request) {
	services := []serviceHealth{
		{Name: "gateway", URL: "http://localhost:" + env("PORT", "8080") + "/healthz"},
		{Name: "auth", URL: env("AUTH_SERVICE_URL", "http://localhost:8081") + "/healthz"},
		{Name: "study", URL: env("STUDY_SERVICE_URL", "http://localhost:8082") + "/healthz"},
		{Name: "quiz", URL: env("QUIZ_SERVICE_URL", "http://localhost:8083") + "/healthz"},
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

// cors chuẩn hóa cho dev – cho phép localhost:5173 và localhost:3000
func cors(next http.Handler) http.Handler {
	allowedOrigins := map[string]bool{
		"http://localhost:5173":   true,
		"http://127.0.0.1:5173":  true,
		"http://localhost:3000":   true,
		"http://127.0.0.1:3000":  true,
		"http://web:5173":         true,
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
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
		log.Printf("[gateway] %s %s -> %d (%s)",
			r.Method, r.URL.Path, rw.status, time.Since(start).Round(time.Millisecond))
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
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "invalid upstream request"})
			return
		}
		req.Header = r.Header.Clone()
		// Ghi upstream info để service biết request từ gateway
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Header.Set("X-Forwarded-Host", r.Host)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream unavailable"})
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
