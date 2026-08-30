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
	mux.HandleFunc("GET /healthz", health("gateway"))
	mux.HandleFunc("GET /healthz/services", servicesHealth)
	mux.HandleFunc("/v1/auth/", reverseProxy(env("AUTH_SERVICE_URL", "http://localhost:8081")))
	mux.HandleFunc("/v1/study-sets", reverseProxy(env("STUDY_SERVICE_URL", "http://localhost:8082")))
	mux.HandleFunc("/v1/study-sets/", reverseProxy(env("STUDY_SERVICE_URL", "http://localhost:8082")))
	mux.HandleFunc("/v1/flashcards/", reverseProxy(env("STUDY_SERVICE_URL", "http://localhost:8082")))
	mux.HandleFunc("/v1/live-sessions", reverseProxy(env("QUIZ_SERVICE_URL", "http://localhost:8083")))
	mux.HandleFunc("/v1/live-sessions/", reverseProxy(env("QUIZ_SERVICE_URL", "http://localhost:8083")))

	log.Printf("gateway listening on :%s", port)
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

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "http://localhost:5173" || origin == "http://127.0.0.1:5173" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")

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
		next.ServeHTTP(w, r)
	})
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
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
