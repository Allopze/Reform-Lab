package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/allopze/reform-lab/apps/api/config"
	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/allopze/reform-lab/apps/api/internal/workers"
	"github.com/allopze/reform-lab/apps/api/internal/workers/audio"
	"github.com/allopze/reform-lab/apps/api/internal/workers/document"
	workerImage "github.com/allopze/reform-lab/apps/api/internal/workers/image"
	"github.com/allopze/reform-lab/apps/api/internal/workers/pdf"
	"github.com/allopze/reform-lab/apps/api/internal/workers/video"
	"github.com/hibiken/asynq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if cfg.RedisURL == "" {
		fmt.Fprintln(os.Stderr, "REDIS_URL is required for standalone worker mode. Without Redis, the server runs an embedded worker.")
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.LogLevel)
	metrics := observability.NewMetrics()
	capabilities.ConfigureFeatureFlags(cfg.DisabledCapabilities, cfg.DisabledEngines)
	flags := capabilities.DefaultFlags.Snapshot()
	if len(flags.DisabledCapabilities) > 0 || len(flags.DisabledEngines) > 0 {
		logger.Warn().Strs("disabled_capabilities", flags.DisabledCapabilities).Strs("disabled_engines", flags.DisabledEngines).Msg("feature flags configured")
	}

	// SQLite database
	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("open database")
	}
	defer db.Close()

	if err := database.Migrate(db, cfg.MigrationsPath); err != nil {
		logger.Fatal().Err(err).Msg("run migrations")
	}

	// Storage
	store, err := storage.NewFilesystem(cfg.StorageBasePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("init storage")
	}

	// Repositories
	jobRepo := repository.NewJobRepository(db)
	artifactRepo := repository.NewArtifactRepository(db)
	auditRepo := repository.NewAuditRepository(db)

	// Queue (for orchestrator to update jobs)
	q, err := queue.NewAsynqQueue(cfg.RedisURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("init redis queue")
	}
	defer q.Close()

	orch := orchestrator.NewService(jobRepo, auditRepo, q)

	// Register conversion engines
	registry := workers.NewRegistry()

	registry.Register("pdf-to-jpg", &pdf.ToImagesEngine{})
	registry.Register("pdf-to-png", &pdf.ToImagesEngine{})
	registry.Register("pdf-to-txt", &pdf.ToTextEngine{})
	registry.Register("pdf-to-docx", &pdf.ToDocxEngine{})

	imgConvert := &workerImage.ConvertEngine{}
	registry.Register("image-to-png", imgConvert)
	registry.Register("image-to-jpg", imgConvert)
	registry.Register("image-to-pdf", &workerImage.ToPDFEngine{})

	registry.Register("doc-to-pdf", &document.ToPDFEngine{})
	registry.Register("doc-to-txt", &document.ToTxtEngine{})
	registry.Register("doc-to-docx", &document.ToDocxEngine{})

	audioConvert := &audio.ConvertEngine{}
	registry.Register("audio-to-mp3", audioConvert)
	registry.Register("audio-to-wav", audioConvert)
	registry.Register("audio-to-ogg", audioConvert)

	videoConvert := &video.ConvertEngine{}
	registry.Register("video-to-mp4", videoConvert)
	registry.Register("video-to-webm", videoConvert)
	registry.Register("video-to-gif", &video.ToGIFEngine{})

	handler := &workers.Handler{
		Registry:    registry,
		Store:       store,
		Artifacts:   artifactRepo,
		Audit:       auditRepo,
		Orch:        orch,
		Logger:      logger,
		Metrics:     metrics,
		ArtifactTTL: time.Duration(cfg.ArtifactTTLHours) * time.Hour,
		ArtifactTTLByFamily: map[domain.FormatFamily]time.Duration{
			domain.FamilyPDF:      time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyPDF]) * time.Hour,
			domain.FamilyImage:    time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyImage]) * time.Hour,
			domain.FamilyDocument: time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyDocument]) * time.Hour,
			domain.FamilyAudio:    time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyAudio]) * time.Hour,
			domain.FamilyVideo:    time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyVideo]) * time.Hour,
		},
	}

	// Asynq server
	redisOpt, _ := asynq.ParseRedisURI(cfg.RedisURL)
	srv := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: 4,
		Queues:      map[string]int{"default": 1},
		Logger:      &asynqLogger{logger: logger},
	})

	// Map all conversion task types to the generic handler via Asynq adapter.
	mux := asynq.NewServeMux()
	for _, capID := range []string{
		"pdf-to-jpg", "pdf-to-png", "pdf-to-txt", "pdf-to-docx",
		"image-to-png", "image-to-jpg", "image-to-pdf",
		"doc-to-pdf", "doc-to-txt", "doc-to-docx",
		"audio-to-mp3", "audio-to-wav", "audio-to-ogg",
		"video-to-mp4", "video-to-webm", "video-to-gif",
	} {
		taskType := "conversion:" + capID
		mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
			return handler.ProcessPayload(ctx, t.Type(), t.Payload())
		})
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info().Msg("shutting down worker")
		srv.Shutdown()
	}()

	logger.Info().Msg("starting worker (Redis mode)")
	if err := srv.Run(mux); err != nil {
		logger.Fatal().Err(err).Msg("worker error")
	}
}

// asynqLogger adapts zerolog to asynq's logger interface.
type asynqLogger struct {
	logger observability.LoggerAlias
}

func (l *asynqLogger) Debug(args ...interface{}) {
	l.logger.Debug().Msg(fmt.Sprint(args...))
}
func (l *asynqLogger) Info(args ...interface{}) {
	l.logger.Info().Msg(fmt.Sprint(args...))
}
func (l *asynqLogger) Warn(args ...interface{}) {
	l.logger.Warn().Msg(fmt.Sprint(args...))
}
func (l *asynqLogger) Error(args ...interface{}) {
	l.logger.Error().Msg(fmt.Sprint(args...))
}
func (l *asynqLogger) Fatal(args ...interface{}) {
	l.logger.Fatal().Msg(fmt.Sprint(args...))
}
