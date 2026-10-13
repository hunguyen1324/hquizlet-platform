// Class service entrypoint.
// Wires config → database → repositories → services → HTTP handler → server.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/client"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/config"
	classhttp "github.com/hunguyen1324/hquizlet-platform/services/class/internal/http"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/repository"
	classservice "github.com/hunguyen1324/hquizlet-platform/services/class/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/class/internal/videostorage"
	"github.com/hunguyen1324/hquizlet-platform/services/class/migrations"
)

func main() {
	cfg := config.Load()

	db := openDatabase(cfg.DatabaseURL)
	defer db.Close()

	if err := migrations.Run(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	// Repositories
	classRepo := repository.NewClassRepository(db)
	memberRepo := repository.NewMemberRepository(db)
	studySetRepo := repository.NewClassStudySetRepository(db)
	activityRepo := repository.NewActivityRepository(db)

	// Internal clients
	studyClient := client.NewStudyClient(cfg.StudyServiceURL, cfg.ClassInternalToken)

	// Services
	classSvc := classservice.NewClassService(classRepo, memberRepo)
	memberSvc := classservice.NewMemberService(classRepo, memberRepo)
	studySetSvc := classservice.NewClassStudySetService(classRepo, memberRepo, studySetRepo, studyClient)
	activitySvc := classservice.NewActivityService(activityRepo, studyClient)

	// HTTP
	mux := http.NewServeMux()
	h := classhttp.New(classSvc, memberSvc, studySetSvc, activitySvc, db)
	if cfg.VideoStorage.Host != "" {
		if len(cfg.VideoTicketSecret) < 32 || cfg.VideoMaxBytes < 1024*1024 || cfg.VideoMaxBytes > 2*1024*1024*1024 {
			log.Fatal("private videos require VIDEO_TICKET_SECRET >= 32 characters and VIDEO_MAX_UPLOAD_MB between 1 and 2048")
		}
		store, err := videostorage.New(cfg.VideoStorage)
		if err != nil {
			log.Fatalf("video storage configuration: %v", err)
		}
		h.ConfigureVideos(store, cfg.VideoTicketSecret, cfg.VideoMaxBytes)
		go h.RunVideoCleanup(context.Background())
	}
	h.Register(mux)

	// All /v1 class resources require a user identity. Health remains public.
	handler := middleware.Chain(mux,
		middleware.RequestID,
		middleware.Logging,
		middleware.RequireUserID,
	)

	log.Printf("[class] listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatal(err)
	}
}

func openDatabase(url string) *sql.DB {
	db, err := sql.Open("pgx", url)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	for attempt := 1; attempt <= 20; attempt++ {
		if err := db.PingContext(ctx); err == nil {
			log.Printf("[class] connected to database")
			return db
		}
		log.Printf("[class] waiting for postgres, attempt %d/20", attempt)
		time.Sleep(time.Second)
	}
	log.Fatal("[class] postgres is not reachable after 20 attempts")
	return db
}
