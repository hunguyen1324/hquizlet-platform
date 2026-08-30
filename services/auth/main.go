package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

type registerRequest struct { Name string `json:"name"`; Email string `json:"email"`; Password string `json:"password"` }
type loginRequest struct { Email string `json:"email"`; Password string `json:"password"` }
type user struct { ID int64 `json:"id"`; Name string `json:"name"`; Email string `json:"email"`; Role string `json:"role"`; CreatedAt time.Time `json:"createdAt"` }
type sessionResponse struct { Authenticated bool `json:"authenticated"`; Token string `json:"token"`; ExpiresAt time.Time `json:"expiresAt"`; User user `json:"user"` }

var errEmailTaken = errors.New("email already registered")
var errInvalidCredential = errors.New("invalid email or password")
var errInvalidSession = errors.New("invalid or expired session")

func main() {
	port := env("PORT", "8081")
	db := openDatabase()
	defer db.Close()
	if err := migrate(db); err != nil { log.Fatal(err) }

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health("auth", db))
	mux.HandleFunc("GET /v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		currentUser, err := userFromRequest(r.Context(), db, r)
		if err != nil { writeJSON(w, http.StatusOK, map[string]any{"authenticated": false, "user": nil}); return }
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": currentUser})
	})
	mux.HandleFunc("POST /v1/auth/register", func(w http.ResponseWriter, r *http.Request) {
		response, err := register(r.Context(), db, r.Body)
		if err != nil { status := http.StatusBadRequest; if errors.Is(err, errEmailTaken) { status = http.StatusConflict }; writeError(w, status, err); return }
		writeJSON(w, http.StatusCreated, response)
	})
	mux.HandleFunc("POST /v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		response, err := login(r.Context(), db, r.Body)
		if err != nil { writeError(w, http.StatusUnauthorized, err); return }
		writeJSON(w, http.StatusOK, response)
	})
	mux.HandleFunc("POST /v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if token := bearerToken(r); token != "" { _, _ = db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash = $1", hashToken(token)) }
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		currentUser, err := userFromRequest(r.Context(), db, r)
		if err != nil { writeError(w, http.StatusUnauthorized, errInvalidSession); return }
		response, err := createSession(r.Context(), db, currentUser)
		if err != nil { writeError(w, http.StatusInternalServerError, err); return }
		writeJSON(w, http.StatusOK, response)
	})

	log.Printf("auth service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, logging(requestID(mux))); err != nil { log.Fatal(err) }
}

func openDatabase() *sql.DB {
	db, err := sql.Open("pgx", env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable"))
	if err != nil { log.Fatal(err) }
	ctx := context.Background()
	for attempt := 1; attempt <= 20; attempt++ { if err := db.PingContext(ctx); err == nil { return db }; log.Printf("waiting for postgres, attempt %d/20", attempt); time.Sleep(time.Second) }
	log.Fatal("postgres is not reachable"); return db
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, image TEXT NOT NULL DEFAULT '', role TEXT NOT NULL DEFAULT 'user', created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE IF NOT EXISTS sessions (id BIGSERIAL PRIMARY KEY, user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash TEXT NOT NULL UNIQUE, expires_at TIMESTAMPTZ NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE INDEX IF NOT EXISTS sessions_token_hash_idx ON sessions(token_hash);
		CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);
	`)
	return err
}

func register(ctx context.Context, db *sql.DB, body io.Reader) (sessionResponse, error) {
	var input registerRequest
	if err := json.NewDecoder(body).Decode(&input); err != nil { return sessionResponse{}, errors.New("invalid JSON body") }
	input.Name = strings.TrimSpace(input.Name); input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	if input.Name == "" { return sessionResponse{}, errors.New("name is required") }
	if !strings.Contains(input.Email, "@") { return sessionResponse{}, errors.New("valid email is required") }
	if len(input.Password) < 6 { return sessionResponse{}, errors.New("password must be at least 6 characters") }
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil { return sessionResponse{}, err }
	var created user
	err = db.QueryRowContext(ctx, `INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING id, name, email, role, created_at`, input.Name, input.Email, string(passwordHash)).Scan(&created.ID, &created.Name, &created.Email, &created.Role, &created.CreatedAt)
	if err != nil { if strings.Contains(err.Error(), "duplicate key") { return sessionResponse{}, errEmailTaken }; return sessionResponse{}, err }
	return createSession(ctx, db, created)
}

func login(ctx context.Context, db *sql.DB, body io.Reader) (sessionResponse, error) {
	var input loginRequest
	if err := json.NewDecoder(body).Decode(&input); err != nil { return sessionResponse{}, errors.New("invalid JSON body") }
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	var current user; var passwordHash string
	err := db.QueryRowContext(ctx, `SELECT id, name, email, role, created_at, password_hash FROM users WHERE email = $1`, input.Email).Scan(&current.ID, &current.Name, &current.Email, &current.Role, &current.CreatedAt, &passwordHash)
	if err != nil { return sessionResponse{}, errInvalidCredential }
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil { return sessionResponse{}, errInvalidCredential }
	return createSession(ctx, db, current)
}

func createSession(ctx context.Context, db *sql.DB, current user) (sessionResponse, error) {
	token, err := randomToken(); if err != nil { return sessionResponse{}, err }
	expiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	_, err = db.ExecContext(ctx, `INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`, current.ID, hashToken(token), expiresAt)
	if err != nil { return sessionResponse{}, err }
	return sessionResponse{Authenticated: true, Token: token, ExpiresAt: expiresAt, User: current}, nil
}

func userFromRequest(ctx context.Context, db *sql.DB, r *http.Request) (user, error) {
	token := bearerToken(r); if token == "" { return user{}, errInvalidSession }
	var current user
	err := db.QueryRowContext(ctx, `SELECT u.id, u.name, u.email, u.role, u.created_at FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.token_hash = $1 AND s.expires_at > now()`, hashToken(token)).Scan(&current.ID, &current.Name, &current.Email, &current.Role, &current.CreatedAt)
	if err != nil { return user{}, errInvalidSession }
	return current, nil
}

func bearerToken(r *http.Request) string { header := r.Header.Get("Authorization"); if !strings.HasPrefix(header, "Bearer ") { return "" }; return strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")) }
func randomToken() (string, error) { var bytes [32]byte; _, err := rand.Read(bytes[:]); return hex.EncodeToString(bytes[:]), err }
func hashToken(token string) string { sum := sha256.Sum256([]byte(token)); return hex.EncodeToString(sum[:]) }
func health(service string, db *sql.DB) http.HandlerFunc { return func(w http.ResponseWriter, r *http.Request) { status := http.StatusOK; body := map[string]string{"service": service, "status": "ok", "database": "ok"}; if err := db.PingContext(r.Context()); err != nil { status = http.StatusServiceUnavailable; body["status"] = "degraded"; body["database"] = "offline" }; writeJSON(w, status, body) } }
func requestID(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { id := r.Header.Get("X-Request-ID"); if id == "" { id = time.Now().UTC().Format("20060102150405.000000000") }; w.Header().Set("X-Request-ID", id); next.ServeHTTP(w, r) }) }
func logging(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { start := time.Now(); next.ServeHTTP(w, r); log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond)) }) }
func writeError(w http.ResponseWriter, status int, err error) { writeJSON(w, status, map[string]string{"error": err.Error()}) }
func writeJSON(w http.ResponseWriter, status int, value any) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(value) }
func env(key, fallback string) string { if value := os.Getenv(key); value != "" { return value }; return fallback }
