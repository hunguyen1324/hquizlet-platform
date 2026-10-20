// Study service entrypoint.
// Wires config → database → repositories → services → HTTP handler → server.
package main

import (
	"context"
	"database/sql"
	"github.com/hunguyen1324/hquizlet-platform/packages/authz"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/hunguyen1324/hquizlet-platform/pkg/cache"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/config"
	studyhttp "github.com/hunguyen1324/hquizlet-platform/services/study/internal/http"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/migration"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/snapshotcache"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/storage"
)

func main() {
	cfg := config.Load()

	db := openDatabase(cfg.DatabaseURL)
	defer db.Close()

	if err := migration.Run(db); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	// Expired snapshots contain answers and should not accumulate indefinitely.
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_, err := db.ExecContext(ctx, `DELETE FROM protected_quiz_sessions WHERE expires_at<now()`)
			if err != nil {
				log.Printf("[study] expired quiz session cleanup failed")
			}
			cancel()
		}
	}()

	if err := authz.StartDailyMetrics(db, "study", `SELECT d::date AS day,'studySessions' AS name,(SELECT count(*) FROM learning_sessions WHERE completed_at>=d AND completed_at<d+interval '1 day') AS value FROM generate_series(current_date-179,current_date,interval '1 day') d`); err != nil {
		log.Fatal(err)
	}

	go repository.RunCompletionOutbox(context.Background(), db, os.Getenv("NATS_URL"))
	// Repositories
	setRepo := repository.NewStudySetRepository(db)
	cardRepo := repository.NewFlashcardRepository(db)
	folderRepo := repository.NewFolderRepository(db)
	progressRepo := repository.NewLearningProgressRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	quizRepo := repository.NewQuizQuestionRepository(db)
	importJobRepo := repository.NewImportJobRepository(db)

	// Services
	setSvc := service.NewStudySetService(setRepo, cardRepo).WithLimiter(repository.NewQuizSessionRepository(db), contentPolicyFromEnv())
	cardSvc := service.NewFlashcardService(setRepo, cardRepo)
	folderSvc := service.NewFolderService(folderRepo, setRepo)
	progressSvc := service.NewProgressService(progressRepo, setRepo, cardRepo).WithLearnerAccess(service.NewLearnerAccess(setRepo, cfg.PaymentServiceURL))
	statsSvc := service.NewStatsService(statsRepo)
	quizSvc := service.NewQuizQuestionService(quizRepo, setRepo)
	importSvc := service.NewImportService(cardRepo, quizRepo, setRepo)
	importBlob, err := openImportBlobStorage(cfg)
	if err != nil {
		log.Fatalf("import storage: %v", err)
	}
	importJobSvc := service.NewImportJobService(importJobRepo, cardRepo, quizRepo, setRepo, importBlob)

	// HTTP
	quizAudio := storage.NewQuizAudio(db, cfg.AudioAllowedHosts)
	// Streaming serves byte ranges: a local copy when cheap, otherwise the
	// object itself; Postgres only answers metadata and acts as recovery.
	quizAudio.WithDiskCache(cfg.AudioCacheDir, cfg.AudioCacheBytes)
	if objects, ok := importBlob.(storage.ObjectRangeStore); ok {
		quizAudio.WithObjectStore(objects)
	}
	if backup, ok := importBlob.(storage.AudioBackup); ok {
		quizAudio.WithBackup(backup)
	}
	go quizAudio.RunSync(context.Background())
	redisCache, err := cache.New(cfg.RedisURL)
	if err != nil {
		log.Printf("[study] warning: failed to connect to redis cache: %v", err)
	}

	// Question content is immutable and content-addressed, so each replica keeps
	// a byte-bounded LRU of snapshots: a page request reads RAM instead of
	// decoding the learner's whole state document.
	snapshots := snapshotcache.New(repository.NewQuizSnapshotRepository(db), snapshotcache.DefaultBytes)
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			n, err := snapshots.DeleteUnreferenced(ctx, time.Now().Add(-48*time.Hour))
			if err != nil {
				log.Printf("[study] snapshot cleanup failed: %v", err)
			} else if n > 0 {
				log.Printf("[study] removed %d unreferenced quiz snapshots", n)
			}
			cancel()
		}
	}()

	// Hot quiz buckets are charged in a shared Redis counter when available, so
	// an answer does not write protected_quiz_limits on every tap. Without Redis
	// the service keeps a per-replica budget in memory (still fail-closed).
	quizSvcProtected := service.NewProtectedQuizService(setRepo, quizRepo, repository.NewQuizSessionRepository(db), snapshots, cfg.PaymentServiceURL)
	if redisCache != nil {
		quizSvcProtected = quizSvcProtected.WithLimiter(service.NewRedisLimiter(redisCache))
	}

	mux := http.NewServeMux()
	studyhttp.New(setSvc, cardSvc, folderSvc, progressSvc, quizSvc, importSvc, importJobSvc, db).WithCache(redisCache).WithStats(statsSvc).WithAdmin(service.NewAdminService(repository.NewAdminRepository(db))).WithProtectedQuiz(
		quizSvcProtected,
		quizAudio,
	).WithViewerMarkSecret(os.Getenv("WATERMARK_SECRET")).Register(mux)

	// All /v1 study resources require a user identity. Health remains public.
	handler := middleware.Chain(mux,
		middleware.RequestID,
		middleware.Logging,
		authz.VerifyIdentity(cfg.AuthServiceURL, func(r *http.Request) bool {
			return strings.HasPrefix(r.URL.Path, "/v1/") && !strings.HasPrefix(r.URL.Path, "/v1/templates/") && r.URL.Path != "/v1/quiz-audio"
		}),
		middleware.StudyWritePolicy(db),
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

func openImportBlobStorage(cfg config.Config) (storage.ImportBlobStorage, error) {
	if cfg.MinIO.Endpoint != "" {
		log.Printf("[study] import blob storage: minio %s bucket=%s", cfg.MinIO.Endpoint, cfg.MinIO.Bucket)
		return storage.NewMinIOImportStorage(storage.MinIOConfig{
			Endpoint:        cfg.MinIO.Endpoint,
			AccessKeyID:     cfg.MinIO.AccessKeyID,
			SecretAccessKey: cfg.MinIO.SecretAccessKey,
			Bucket:          cfg.MinIO.Bucket,
		})
	}
	log.Printf("[study] import blob storage: local spool %s", cfg.MinIO.LocalSpoolDir)
	return storage.NewLocalSpoolStorage(cfg.MinIO.LocalSpoolDir)
}
