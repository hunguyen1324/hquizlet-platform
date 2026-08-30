package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/config"
	authhttp "github.com/hunguyen1324/hquizlet-platform/services/auth/internal/http"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/migration"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/auth/internal/service"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.Load()

	db := openDatabase(cfg.DatabaseURL)
	defer db.Close()

	if err := migration.Run(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	repo := repository.New(db)
	svc := service.New(repo, cfg.SessionTTL)
	router := authhttp.NewRouter(svc, db)

	log.Printf("[auth] listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func openDatabase(dsn string) *sql.DB {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("db open: %v", err)
	}
	ctx := context.Background()
	for i := 1; i <= 20; i++ {
		if err := db.PingContext(ctx); err == nil {
			return db
		}
		log.Printf("[auth] waiting for postgres (%d/20)…", i)
		time.Sleep(time.Second)
	}
	log.Fatal("[auth] postgres unreachable after 20 attempts")
	return db
}
