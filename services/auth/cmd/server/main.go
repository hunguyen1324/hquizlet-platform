package main

import (
	"context"
	"database/sql"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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

	if err := authz.StartDailyMetrics(db, "auth", `SELECT d::date AS day,'newUsers' AS name,(SELECT count(*) FROM users WHERE created_at>=d AND created_at<d+interval '1 day') AS value FROM generate_series(current_date-179,current_date,interval '1 day') d UNION ALL SELECT d::date,'activeUsers',(SELECT count(DISTINCT user_id) FROM sessions WHERE created_at>=d AND created_at<d+interval '1 day') FROM generate_series(current_date-179,current_date,interval '1 day') d`); err != nil {
		log.Fatal(err)
	}

	repo := repository.New(db)
	if err := repo.SeedManagedRoles(context.Background()); err != nil {
		log.Fatalf("iam role seed failed: %v", err)
	}
	svc := service.New(repo, cfg.SessionTTL).WithOverview(repository.NewOverviewRepository(db))
	authURL := os.Getenv("AUTH_SERVICE_URL")
	if authURL == "" {
		authURL = "http://localhost:" + cfg.Port
	}
	baseRouter := authhttp.NewRouter(svc, db)
	router := authz.VerifyIdentity(authURL, func(r *http.Request) bool { return strings.HasPrefix(r.URL.Path, "/v1/admin/") })(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/admin/") {
			identity, err := svc.VerifyToken(r.Context(), strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
			if err != nil {
				http.Error(w, "authentication required", 401)
				return
			}
			r.Header.Set("X-User-ID", strconv.FormatInt(identity.UserID, 10))
			r.Header.Set("X-User-Role", identity.Role)
			r.Header.Set("X-User-Roles", strings.Join(identity.PlatformRoles, ","))
		}
		baseRouter.ServeHTTP(w, r)
	}))

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
