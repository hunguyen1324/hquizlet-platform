package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/config"
	filehttp "github.com/hunguyen1324/hquizlet-platform/services/file/internal/http"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/storage"
	"github.com/hunguyen1324/hquizlet-platform/services/file/internal/store"
)

func main() {
	cfg := config.Load()

	if err := cfg.Validate(); err != nil {
		log.Fatalf("[file] config error: %v", err)
	}

	// Database
	db := store.Open(cfg.DatabaseURL)
	defer db.Close()

	if err := store.RunMigrations(db); err != nil {
		log.Fatalf("[file] migration failed: %v", err)
	}

	// Storage client
	storageClient, err := storage.New(cfg.Storage)
	if err != nil {
		log.Fatalf("[file] storage init: %v", err)
	}

	// Wire dependencies
	repo := repository.New(db)
	svc := service.New(repo, storageClient, cfg)
	router := filehttp.NewRouter(svc, db)

	log.Printf("[file] listening on :%s (storage: %s)", cfg.Port, cfg.Storage.Provider)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("[file] server error: %v", err)
	}
}

func openDatabase(dsn string) *sql.DB {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("[file] db open: %v", err)
	}
	ctx := context.Background()
	for i := 1; i <= 20; i++ {
		if err := db.PingContext(ctx); err == nil {
			return db
		}
		log.Printf("[file] waiting for postgres (%d/20)…", i)
		time.Sleep(time.Second)
	}
	log.Fatal("[file] postgres unreachable after 20 attempts")
	return db
}
