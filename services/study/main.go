package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type studySet struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

func main() {
	port := env("PORT", "8082")
	db := openDatabase()
	defer db.Close()

	if err := migrate(db); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health("study", db))
	mux.HandleFunc("GET /v1/study-sets", func(w http.ResponseWriter, r *http.Request) {
		sets, err := listStudySets(r.Context(), db)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load study sets"})
			return
		}
		writeJSON(w, http.StatusOK, sets)
	})

	log.Printf("study service listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func openDatabase() *sql.DB {
	databaseURL := env("DATABASE_URL", "postgres://hquizlet:hquizlet@localhost:5432/hquizlet?sslmode=disable")
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	for attempt := 1; attempt <= 20; attempt++ {
		if err := db.PingContext(ctx); err == nil {
			return db
		}
		log.Printf("waiting for postgres, attempt %d/20", attempt)
		time.Sleep(time.Second)
	}

	log.Fatal("postgres is not reachable")
	return db
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS study_sets (
			id BIGSERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		INSERT INTO study_sets (title, description)
		SELECT 'Go + Rust migration basics', 'First demo study set stored in PostgreSQL'
		WHERE NOT EXISTS (SELECT 1 FROM study_sets);
	`)
	return err
}

func listStudySets(ctx context.Context, db *sql.DB) ([]studySet, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, description, created_at
		FROM study_sets
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sets := []studySet{}
	for rows.Next() {
		var set studySet
		if err := rows.Scan(&set.ID, &set.Title, &set.Description, &set.CreatedAt); err != nil {
			return nil, err
		}
		sets = append(sets, set)
	}

	return sets, rows.Err()
}

func health(service string, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		body := map[string]string{"service": service, "status": "ok", "database": "ok"}

		if err := db.PingContext(r.Context()); err != nil {
			status = http.StatusServiceUnavailable
			body["status"] = "degraded"
			body["database"] = "offline"
		}

		writeJSON(w, status, body)
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
