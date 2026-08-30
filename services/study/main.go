package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type studySet struct { ID int64 `json:"id"`; Title string `json:"title"`; Description string `json:"description"`; CreatedAt time.Time `json:"createdAt"`; UpdatedAt time.Time `json:"updatedAt"`; Flashcards []flashcard `json:"flashcards,omitempty"` }
type flashcard struct { ID int64 `json:"id"`; StudySetID int64 `json:"studySetId"`; Term string `json:"term"`; Definition string `json:"definition"`; Starred bool `json:"starred"`; CreatedAt time.Time `json:"createdAt"`; UpdatedAt time.Time `json:"updatedAt"` }
type setRequest struct { Title string `json:"title"`; Description string `json:"description"` }
type cardRequest struct { Term string `json:"term"`; Definition string `json:"definition"` }

var errNotFound = errors.New("resource not found")

func main() {
	port := env("PORT", "8082")
	db := openDatabase(); defer db.Close()
	if err := migrate(db); err != nil { log.Fatal(err) }
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health("study", db))
	mux.HandleFunc("GET /v1/study-sets", func(w http.ResponseWriter, r *http.Request) { sets, err := listStudySets(r.Context(), db); if err != nil { writeError(w, http.StatusInternalServerError, "failed to load study sets"); return }; writeJSON(w, http.StatusOK, sets) })
	mux.HandleFunc("POST /v1/study-sets", func(w http.ResponseWriter, r *http.Request) { set, err := createStudySet(r.Context(), db, r.Body); if err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }; writeJSON(w, http.StatusCreated, set) })
	mux.HandleFunc("/v1/study-sets/", func(w http.ResponseWriter, r *http.Request) { handleStudySet(w, r, db) })
	mux.HandleFunc("/v1/flashcards/", func(w http.ResponseWriter, r *http.Request) { handleFlashcard(w, r, db) })
	log.Printf("study service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, logging(requestID(mux))); err != nil { log.Fatal(err) }
}

func handleStudySet(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	parts := pathParts(r.URL.Path, "/v1/study-sets/"); if len(parts) == 0 { writeError(w, http.StatusNotFound, "study set not found"); return }
	setID, err := strconv.ParseInt(parts[0], 10, 64); if err != nil { writeError(w, http.StatusBadRequest, "invalid study set id"); return }
	if len(parts) == 2 && parts[1] == "flashcards" && r.Method == http.MethodPost { card, err := createFlashcard(r.Context(), db, setID, r.Body); if err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }; writeJSON(w, http.StatusCreated, card); return }
	switch r.Method {
	case http.MethodGet:
		set, err := getStudySet(r.Context(), db, setID); if err != nil { status := http.StatusInternalServerError; if errors.Is(err, errNotFound) { status = http.StatusNotFound }; writeError(w, status, err.Error()); return }; writeJSON(w, http.StatusOK, set)
	case http.MethodPut:
		set, err := updateStudySet(r.Context(), db, setID, r.Body); if err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }; writeJSON(w, http.StatusOK, set)
	case http.MethodDelete:
		if err := deleteStudySet(r.Context(), db, setID); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }; writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func handleFlashcard(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	parts := pathParts(r.URL.Path, "/v1/flashcards/"); if len(parts) == 0 { writeError(w, http.StatusNotFound, "flashcard not found"); return }
	cardID, err := strconv.ParseInt(parts[0], 10, 64); if err != nil { writeError(w, http.StatusBadRequest, "invalid flashcard id"); return }
	if len(parts) == 2 && parts[1] == "star" && r.Method == http.MethodPost { card, err := toggleStar(r.Context(), db, cardID); if err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }; writeJSON(w, http.StatusOK, card); return }
	switch r.Method {
	case http.MethodPut:
		card, err := updateFlashcard(r.Context(), db, cardID, r.Body); if err != nil { writeError(w, http.StatusBadRequest, err.Error()); return }; writeJSON(w, http.StatusOK, card)
	case http.MethodDelete:
		if err := deleteFlashcard(r.Context(), db, cardID); err != nil { writeError(w, http.StatusInternalServerError, err.Error()); return }; writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func openDatabase() *sql.DB { db, err := sql.Open("pgx", env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable")); if err != nil { log.Fatal(err) }; ctx := context.Background(); for attempt := 1; attempt <= 20; attempt++ { if err := db.PingContext(ctx); err == nil { return db }; log.Printf("waiting for postgres, attempt %d/20", attempt); time.Sleep(time.Second) }; log.Fatal("postgres is not reachable"); return db }
func migrate(db *sql.DB) error { _, err := db.Exec(`CREATE TABLE IF NOT EXISTS study_sets (id BIGSERIAL PRIMARY KEY, user_id BIGINT, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()); CREATE TABLE IF NOT EXISTS flashcards (id BIGSERIAL PRIMARY KEY, study_set_id BIGINT NOT NULL REFERENCES study_sets(id) ON DELETE CASCADE, term TEXT NOT NULL, definition TEXT NOT NULL, starred BOOLEAN NOT NULL DEFAULT false, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()); CREATE INDEX IF NOT EXISTS flashcards_study_set_id_idx ON flashcards(study_set_id); INSERT INTO study_sets (title, description) SELECT 'Go + Rust migration basics', 'First demo study set stored in PostgreSQL' WHERE NOT EXISTS (SELECT 1 FROM study_sets);`); return err }
func listStudySets(ctx context.Context, db *sql.DB) ([]studySet, error) { rows, err := db.QueryContext(ctx, `SELECT s.id, s.title, s.description, s.created_at, s.updated_at, COUNT(f.id) FROM study_sets s LEFT JOIN flashcards f ON f.study_set_id = s.id GROUP BY s.id ORDER BY s.updated_at DESC, s.id DESC`); if err != nil { return nil, err }; defer rows.Close(); sets := []studySet{}; for rows.Next() { var set studySet; var count int; if err := rows.Scan(&set.ID, &set.Title, &set.Description, &set.CreatedAt, &set.UpdatedAt, &count); err != nil { return nil, err }; sets = append(sets, set) }; return sets, rows.Err() }
func getStudySet(ctx context.Context, db *sql.DB, id int64) (studySet, error) { var set studySet; err := db.QueryRowContext(ctx, `SELECT id, title, description, created_at, updated_at FROM study_sets WHERE id = $1`, id).Scan(&set.ID, &set.Title, &set.Description, &set.CreatedAt, &set.UpdatedAt); if err != nil { if errors.Is(err, sql.ErrNoRows) { return studySet{}, errNotFound }; return studySet{}, err }; cards, err := listFlashcards(ctx, db, id); if err != nil { return studySet{}, err }; set.Flashcards = cards; return set, nil }
func createStudySet(ctx context.Context, db *sql.DB, body io.Reader) (studySet, error) { input, err := decodeSet(body); if err != nil { return studySet{}, err }; var set studySet; err = db.QueryRowContext(ctx, `INSERT INTO study_sets (title, description) VALUES ($1, $2) RETURNING id, title, description, created_at, updated_at`, input.Title, input.Description).Scan(&set.ID, &set.Title, &set.Description, &set.CreatedAt, &set.UpdatedAt); return set, err }
func updateStudySet(ctx context.Context, db *sql.DB, id int64, body io.Reader) (studySet, error) { input, err := decodeSet(body); if err != nil { return studySet{}, err }; var set studySet; err = db.QueryRowContext(ctx, `UPDATE study_sets SET title = $1, description = $2, updated_at = now() WHERE id = $3 RETURNING id, title, description, created_at, updated_at`, input.Title, input.Description, id).Scan(&set.ID, &set.Title, &set.Description, &set.CreatedAt, &set.UpdatedAt); return set, err }
func deleteStudySet(ctx context.Context, db *sql.DB, id int64) error { _, err := db.ExecContext(ctx, "DELETE FROM study_sets WHERE id = $1", id); return err }
func listFlashcards(ctx context.Context, db *sql.DB, studySetID int64) ([]flashcard, error) { rows, err := db.QueryContext(ctx, `SELECT id, study_set_id, term, definition, starred, created_at, updated_at FROM flashcards WHERE study_set_id = $1 ORDER BY id ASC`, studySetID); if err != nil { return nil, err }; defer rows.Close(); cards := []flashcard{}; for rows.Next() { var card flashcard; if err := rows.Scan(&card.ID, &card.StudySetID, &card.Term, &card.Definition, &card.Starred, &card.CreatedAt, &card.UpdatedAt); err != nil { return nil, err }; cards = append(cards, card) }; return cards, rows.Err() }
func createFlashcard(ctx context.Context, db *sql.DB, studySetID int64, body io.Reader) (flashcard, error) { input, err := decodeCard(body); if err != nil { return flashcard{}, err }; var card flashcard; err = db.QueryRowContext(ctx, `INSERT INTO flashcards (study_set_id, term, definition) VALUES ($1, $2, $3) RETURNING id, study_set_id, term, definition, starred, created_at, updated_at`, studySetID, input.Term, input.Definition).Scan(&card.ID, &card.StudySetID, &card.Term, &card.Definition, &card.Starred, &card.CreatedAt, &card.UpdatedAt); return card, err }
func updateFlashcard(ctx context.Context, db *sql.DB, id int64, body io.Reader) (flashcard, error) { input, err := decodeCard(body); if err != nil { return flashcard{}, err }; var card flashcard; err = db.QueryRowContext(ctx, `UPDATE flashcards SET term = $1, definition = $2, updated_at = now() WHERE id = $3 RETURNING id, study_set_id, term, definition, starred, created_at, updated_at`, input.Term, input.Definition, id).Scan(&card.ID, &card.StudySetID, &card.Term, &card.Definition, &card.Starred, &card.CreatedAt, &card.UpdatedAt); return card, err }
func toggleStar(ctx context.Context, db *sql.DB, id int64) (flashcard, error) { var card flashcard; err := db.QueryRowContext(ctx, `UPDATE flashcards SET starred = NOT starred, updated_at = now() WHERE id = $1 RETURNING id, study_set_id, term, definition, starred, created_at, updated_at`, id).Scan(&card.ID, &card.StudySetID, &card.Term, &card.Definition, &card.Starred, &card.CreatedAt, &card.UpdatedAt); return card, err }
func deleteFlashcard(ctx context.Context, db *sql.DB, id int64) error { _, err := db.ExecContext(ctx, "DELETE FROM flashcards WHERE id = $1", id); return err }
func decodeSet(body io.Reader) (setRequest, error) { var input setRequest; if err := json.NewDecoder(body).Decode(&input); err != nil { return setRequest{}, errors.New("invalid JSON body") }; input.Title = strings.TrimSpace(input.Title); input.Description = strings.TrimSpace(input.Description); if input.Title == "" { return setRequest{}, errors.New("title is required") }; return input, nil }
func decodeCard(body io.Reader) (cardRequest, error) { var input cardRequest; if err := json.NewDecoder(body).Decode(&input); err != nil { return cardRequest{}, errors.New("invalid JSON body") }; input.Term = strings.TrimSpace(input.Term); input.Definition = strings.TrimSpace(input.Definition); if input.Term == "" || input.Definition == "" { return cardRequest{}, errors.New("term and definition are required") }; return input, nil }
func pathParts(path string, prefix string) []string { raw := strings.Trim(strings.TrimPrefix(path, prefix), "/"); if raw == "" { return nil }; return strings.Split(raw, "/") }
func health(service string, db *sql.DB) http.HandlerFunc { return func(w http.ResponseWriter, r *http.Request) { status := http.StatusOK; body := map[string]string{"service": service, "status": "ok", "database": "ok"}; if err := db.PingContext(r.Context()); err != nil { status = http.StatusServiceUnavailable; body["status"] = "degraded"; body["database"] = "offline" }; writeJSON(w, status, body) } }
func requestID(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { id := r.Header.Get("X-Request-ID"); if id == "" { id = time.Now().UTC().Format("20060102150405.000000000") }; w.Header().Set("X-Request-ID", id); next.ServeHTTP(w, r) }) }
func logging(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { start := time.Now(); next.ServeHTTP(w, r); log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond)) }) }
func writeError(w http.ResponseWriter, status int, message string) { writeJSON(w, status, map[string]string{"error": message}) }
func writeJSON(w http.ResponseWriter, status int, value any) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(value) }
func env(key, fallback string) string { if value := os.Getenv(key); value != "" { return value }; return fallback }
