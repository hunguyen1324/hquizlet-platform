// Study service entrypoint.
// Wires config → database → repositories → services → HTTP handler → server.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/config"
	studyhttp "github.com/hunguyen1324/hquizlet-platform/services/study/internal/http"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/migration"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
)

func main() {
	cfg := config.Load()

	db := openDatabase(cfg.DatabaseURL)
	defer db.Close()

	if err := migration.Run(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	// Repositories
	setRepo := repository.NewStudySetRepository(db)
	cardRepo := repository.NewFlashcardRepository(db)

	// Services
	setSvc := service.NewStudySetService(setRepo, cardRepo)
	cardSvc := service.NewFlashcardService(setRepo, cardRepo)

	// HTTP
	mux := http.NewServeMux()
	studyhttp.New(setSvc, cardSvc, db).Register(mux)

	handler := middleware.Chain(mux,
		middleware.RequestID,
		middleware.Logging,
	)

	log.Printf("[study] listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal(err)
	}
}

// openDatabase connects to PostgreSQL and retries until it is reachable.
func openDatabase(url string) *sql.DB {
	db, err := sql.Open("pgx", url)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	for attempt := 1; attempt <= 20; attempt++ {
		if err := db.PingContext(ctx); err == nil {
			log.Printf("[study] connected to database")
			return db
		}
		log.Printf("[study] waiting for postgres, attempt %d/20", attempt)
		time.Sleep(time.Second)
	}
	log.Fatal("[study] postgres is not reachable after 20 attempts")
	return db
}
