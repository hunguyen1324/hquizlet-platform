// Study service entrypoint.
// Wires config → database → repositories → services → HTTP handler → server.
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/config"
	studyhttp "github.com/hunguyen1324/hquizlet-platform/services/study/internal/http"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/middleware"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/migration"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/repository"
	"github.com/hunguyen1324/hquizlet-platform/services/study/internal/service"
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

	// Repositories
	setRepo := repository.NewStudySetRepository(db)
	cardRepo := repository.NewFlashcardRepository(db)
	folderRepo := repository.NewFolderRepository(db)
	progressRepo := repository.NewLearningProgressRepository(db)
	quizRepo := repository.NewQuizQuestionRepository(db)
	importJobRepo := repository.NewImportJobRepository(db)

	// Services
	setSvc := service.NewStudySetService(setRepo, cardRepo).WithLimiter(repository.NewQuizSessionRepository(db), contentPolicyFromEnv())
	cardSvc := service.NewFlashcardService(setRepo, cardRepo)
	folderSvc := service.NewFolderService(folderRepo, setRepo)
	progressSvc := service.NewProgressService(progressRepo, setRepo, cardRepo)
	quizSvc := service.NewQuizQuestionService(quizRepo, setRepo)
	importSvc := service.NewImportService(cardRepo, quizRepo, setRepo)
	importBlob, err := openImportBlobStorage(cfg)
	if err != nil {
		log.Fatalf("import storage: %v", err)
	}
	importJobSvc := service.NewImportJobService(importJobRepo, cardRepo, quizRepo, setRepo, importBlob)

	// HTTP
	quizAudio := storage.NewQuizAudio(db, cfg.AudioAllowedHosts)
	if backup, ok := importBlob.(storage.AudioBackup); ok {
		quizAudio.WithBackup(backup)
	}
	go quizAudio.RunSync(context.Background())
	mux := http.NewServeMux()
	studyhttp.New(setSvc, cardSvc, folderSvc, progressSvc, quizSvc, importSvc, importJobSvc, db).WithProtectedQuiz(
		service.NewProtectedQuizService(setRepo, quizRepo, repository.NewQuizSessionRepository(db), cfg.PaymentServiceURL),
		quizAudio,
	).WithViewerMarkSecret(os.Getenv("WATERMARK_SECRET")).Register(mux)

	// All /v1 study resources require a user identity. Health remains public.
	handler := middleware.Chain(mux,
		middleware.RequestID,
		middleware.Logging,
		middleware.VerifyBearer(cfg.AuthServiceURL),
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
