package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/allopze/reform-lab/apps/api/config"
	"github.com/allopze/reform-lab/apps/api/internal/api"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
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
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.LogLevel)
	metrics := observability.NewMetrics()
	capabilities.ConfigureFeatureFlags(cfg.DisabledCapabilities, cfg.DisabledEngines)
	flags := capabilities.DefaultFlags.Snapshot()
	if len(flags.DisabledCapabilities) > 0 || len(flags.DisabledEngines) > 0 {
		logger.Warn().Strs("disabled_capabilities", flags.DisabledCapabilities).Strs("disabled_engines", flags.DisabledEngines).Msg("feature flags configured")
	}

	// Probe runtime engine availability once at startup.
	capabilities.DefaultProber.Probe()
	for engine, ok := range capabilities.DefaultProber.AvailableEngines() {
		logger.Info().Str("engine", engine).Bool("available", ok).Msg("engine probe")
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
	fileRepo := repository.NewFileRepository(db)
	jobRepo := repository.NewJobRepository(db)
	artifactRepo := repository.NewArtifactRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	userRepo := repository.NewUserRepository(db)
	dashboardRepo := repository.NewDashboardRepository(db)

	// Auth
	authSvc := auth.NewService(userRepo, cfg.JWTSecret)

	// Queue — use Redis if configured, otherwise in-process with embedded worker.
	var jobQueue queue.JobQueue
	var workerHandler *workers.Handler

	if cfg.RedisURL != "" {
		q, err := queue.NewAsynqQueue(cfg.RedisURL)
		if err != nil {
			logger.Fatal().Err(err).Msg("init redis queue")
		}
		defer q.Close()
		jobQueue = q
		logger.Info().Msg("using Redis queue")
	} else {
		// Build embedded worker for in-process mode.
		registry := buildRegistry()
		workerHandler = &workers.Handler{
			Registry:    registry,
			Store:       store,
			Artifacts:   artifactRepo,
			Audit:       auditRepo,
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
		ipq := queue.NewInProcessQueue(workerHandler.ProcessPayload)
		defer ipq.Close()
		jobQueue = ipq
		logger.Info().Msg("using in-process queue (no Redis)")
	}

	// Orchestrator — single instance shared by router and worker handler.
	orch := orchestrator.NewService(jobRepo, auditRepo, jobQueue)
	if workerHandler != nil {
		workerHandler.Orch = orch
	}

	retentionCtx, stopRetention := context.WithCancel(context.Background())
	defer stopRetention()
	retention := orchestrator.NewRetentionService(artifactRepo, jobRepo, logger)
	go retention.Start(retentionCtx, 15*time.Minute)

	// Router
	router := api.NewRouter(api.Deps{
		Logger:           logger,
		Metrics:          metrics,
		Store:            store,
		Files:            fileRepo,
		Jobs:             jobRepo,
		Artifacts:        artifactRepo,
		Audit:            auditRepo,
		Users:            userRepo,
		Dashboard:        dashboardRepo,
		Orchestrator:     orch,
		AuthService:      authSvc,
		CORSOrigin:       cfg.CORSOrigin,
		ArtifactTTLHours: cfg.ArtifactTTLHours,
		ArtifactTTLByFamily: map[string]int{
			"pdf":      cfg.ArtifactTTLByFamily[domain.FamilyPDF],
			"image":    cfg.ArtifactTTLByFamily[domain.FamilyImage],
			"document": cfg.ArtifactTTLByFamily[domain.FamilyDocument],
			"audio":    cfg.ArtifactTTLByFamily[domain.FamilyAudio],
			"video":    cfg.ArtifactTTLByFamily[domain.FamilyVideo],
		},
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info().Msg("shutting down server")
		stopRetention()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	logger.Info().Int("port", cfg.Port).Msg("starting API server")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal().Err(err).Msg("server error")
	}
}

func buildRegistry() *workers.Registry {
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

	return registry
}
