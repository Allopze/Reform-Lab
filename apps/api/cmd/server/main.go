package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/allopze/reform-lab/apps/api/config"
	"github.com/allopze/reform-lab/apps/api/internal/api"
	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/security"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	webhookpkg "github.com/allopze/reform-lab/apps/api/internal/webhook"
	"github.com/allopze/reform-lab/apps/api/internal/workers"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	logger := observability.NewLogger(cfg.LogLevel)
	metrics := observability.NewMetrics()

	if cfg.RedisURL != "" {
		capabilities.DefaultProber = capabilities.NewDeclaredEngineProber()
	}

	shutdownTracer, err := observability.InitTracer(context.Background(), "reform-api", "1.0.0")
	if err != nil {
		logger.Fatal().Err(err).Msg("init tracer")
	}
	defer shutdownTracer(context.Background())

	capabilities.ConfigureFeatureFlags(cfg.DisabledCapabilities, cfg.DisabledEngines)
	flags := capabilities.DefaultFlags.Snapshot()
	if len(flags.DisabledCapabilities) > 0 || len(flags.DisabledEngines) > 0 {
		logger.Warn().Strs("disabled_capabilities", flags.DisabledCapabilities).Strs("disabled_engines", flags.DisabledEngines).Msg("feature flags configured")
	}

	// Probe runtime engine availability once at startup unless we are in
	// production API mode, where capabilities are declared and execution runs in
	// the worker container.
	capabilities.DefaultProber.Probe()
	unavailableEngines := []string{}
	for engine, ok := range capabilities.DefaultProber.AvailableEngines() {
		if ok {
			logger.Info().Str("engine", engine).Msg("engine available")
		} else {
			logger.Warn().Str("engine", engine).Msg("engine NOT available — related capabilities will be hidden")
			unavailableEngines = append(unavailableEngines, engine)
		}
	}
	if len(unavailableEngines) > 0 {
		logger.Warn().Strs("unavailable", unavailableEngines).Msg("some engines are not installed; capabilities depending on them will not be offered to users")
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
	storageFS, err := storage.NewFilesystem(cfg.StorageBasePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("init storage")
	}
	store := storage.Store(storageFS)
	secretKeeper, err := security.NewSecretKeeper(cfg.SecretEncryptionKey)
	if err != nil {
		logger.Fatal().Err(err).Msg("init secret keeper")
	}

	// Repositories
	fileRepo := repository.NewFileRepository(db)
	jobRepo := repository.NewJobRepository(db)
	artifactRepo := repository.NewArtifactRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	userRepo := repository.NewUserRepository(db)
	passwordResetRepo := repository.NewPasswordResetRepository(db)
	emailVerificationRepo := repository.NewEmailVerificationRepository(db)
	dashboardRepo := repository.NewDashboardRepository(db)
	workerStatusRepo := repository.NewWorkerStatusRepository(db)
	runtimeControlRepo := repository.NewRuntimeControlRepository(db)
	siteSettingRepo := repository.NewSiteSettingRepository(db)
	emailTemplateRepo := repository.NewEmailTemplateRepository(db)
	webhookRepo := repository.NewWebhookRepository(db, repository.WithSecretKeeper(secretKeeper))

	// Auth
	authSvc := auth.NewService(
		userRepo,
		cfg.JWTSecret,
		auth.WithExplicitBootstrapRequired(cfg.AppEnv == "production"),
		auth.WithBootstrapAdminEmails(cfg.BootstrapAdminEmails),
	)

	// Email
	emailSvc := email.NewService(cfg, siteSettingRepo, emailTemplateRepo, logger, email.WithSecretKeeper(secretKeeper))
	emailHandler := &workers.EmailHandler{
		Email:  emailSvc,
		Logger: logger.With().Str("component", "email_worker").Logger(),
	}
	webhookHandler := &workers.WebhookHandler{
		Webhooks: webhookRepo,
		Logger:   logger.With().Str("component", "webhook_worker").Logger(),
	}

	// Queue — use Redis if configured, otherwise in-process with embedded worker.
	var jobQueue queue.JobQueue
	var workerHandler *workers.Handler
	queueMode := "in-process"
	effectiveWorkerConcurrency := cfg.InProcessConcurrency
	runtimeMode := "embedded"
	workerID := "embedded-" + strconv.Itoa(os.Getpid())

	if cfg.RedisURL != "" {
		q, err := queue.NewAsynqQueue(cfg.RedisURL)
		if err != nil {
			logger.Fatal().Err(err).Msg("init redis queue")
		}
		defer q.Close()
		jobQueue = q
		queueMode = "redis"
		runtimeMode = "server"
		effectiveWorkerConcurrency = cfg.WorkerConcurrency
		logger.Info().Msg("using Redis queue")
	} else {
		// Build embedded worker for in-process mode.
		registry := buildRegistry()
		workerHandler = &workers.Handler{
			Registry:     registry,
			Store:        store,
			Artifacts:    artifactRepo,
			Audit:        auditRepo,
			Logger:       logger,
			Metrics:      metrics,
			WorkerStatus: workerStatusRepo,
			WorkerID:     workerID,
			RuntimeMode:  runtimeMode,
			QueueMode:    queueMode,
			ArtifactTTL:  time.Duration(cfg.ArtifactTTLHours) * time.Hour,
			ArtifactTTLByFamily: map[domain.FormatFamily]time.Duration{
				domain.FamilyPDF:      time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyPDF]) * time.Hour,
				domain.FamilyImage:    time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyImage]) * time.Hour,
				domain.FamilyDocument: time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyDocument]) * time.Hour,
				domain.FamilyAudio:    time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyAudio]) * time.Hour,
				domain.FamilyVideo:    time.Duration(cfg.ArtifactTTLByFamily[domain.FamilyVideo]) * time.Hour,
			},
		}

		// Multiplex: route email tasks to emailHandler, everything else to conversion handler.
		dispatcher := func(ctx context.Context, taskType string, payload []byte) error {
			if taskType == queue.EmailTaskType {
				return emailHandler.ProcessPayload(ctx, taskType, payload)
			}
			if taskType == queue.WebhookTaskType {
				return webhookHandler.ProcessPayload(ctx, taskType, payload)
			}
			return workerHandler.ProcessPayload(ctx, taskType, payload)
		}
		ipq := queue.NewInProcessQueueWithLimit(dispatcher, cfg.InProcessConcurrency)
		defer ipq.Close()
		jobQueue = ipq
		logger.Warn().
			Str("app_env", cfg.AppEnv).
			Int("concurrency", cfg.InProcessConcurrency).
			Msg("using in-process queue (development mode only)")
	}

	// Orchestrator — single instance shared by router and worker handler.
	emailNotifier := email.NewJobNotifier(cfg, emailSvc, jobQueue, userRepo, fileRepo, logger)
	webhookNotifier := webhookpkg.NewNotifier(jobQueue, webhookRepo, fileRepo, logger)
	notifier := orchestrator.NewMultiNotifier(emailNotifier, webhookNotifier)
	orch := orchestrator.NewService(
		jobRepo,
		auditRepo,
		jobQueue,
		orchestrator.WithMaxActiveJobsPerUser(cfg.MaxActiveJobsPerUser),
		orchestrator.WithMaxActiveJobsPerGuestSession(cfg.MaxActiveJobsPerGuestSession),
		orchestrator.WithNotifier(notifier),
		orchestrator.WithRuntimeControls(runtimeControlRepo),
	)
	retentionCtx, stopRetention := context.WithCancel(context.Background())
	defer stopRetention()
	if workerHandler != nil {
		workerHandler.Orch = orch
		workers.StartHeartbeatLoop(retentionCtx, workerStatusRepo, workerID, runtimeMode, queueMode, 10*time.Second)
	}
	retention := orchestrator.NewRetentionService(artifactRepo, jobRepo, logger)
	go retention.Start(retentionCtx, 15*time.Minute)
	storageCleanup := storage.NewCleanupService(storageFS.BasePath(), logger, time.Duration(cfg.OriginalTTLHours)*time.Hour, time.Duration(cfg.TempTTLHours)*time.Hour).
		WithFileExpirer(fileRepo)
	go storageCleanup.Start(retentionCtx, 30*time.Minute)

	// Periodically update disk pressure gauges.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			if free, total, err := storageFS.DiskStats(); err == nil {
				metrics.DiskFreeBytes.Set(float64(free))
				metrics.DiskTotalBytes.Set(float64(total))
			}
			select {
			case <-retentionCtx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	// Router
	router := api.NewRouter(api.Deps{
		Logger:                                  logger,
		Metrics:                                 metrics,
		Database:                                db,
		StorageBasePath:                         cfg.StorageBasePath,
		Store:                                   store,
		Files:                                   fileRepo,
		Jobs:                                    jobRepo,
		Artifacts:                               artifactRepo,
		Audit:                                   auditRepo,
		Users:                                   userRepo,
		PasswordResets:                          passwordResetRepo,
		EmailVerifications:                      emailVerificationRepo,
		Dashboard:                               dashboardRepo,
		Workers:                                 workerStatusRepo,
		RuntimeControls:                         runtimeControlRepo,
		SiteSettings:                            siteSettingRepo,
		EmailTemplates:                          emailTemplateRepo,
		Webhooks:                                webhookRepo,
		EmailService:                            emailSvc,
		SecretKeeper:                            secretKeeper,
		Queue:                                   jobQueue,
		Orchestrator:                            orch,
		AuthService:                             authSvc,
		AppURL:                                  cfg.AppURL,
		CORSOrigin:                              cfg.CORSOrigin,
		ExposeMetrics:                           cfg.ExposeMetrics,
		MetricsToken:                            cfg.MetricsToken,
		TrustProxyHeaders:                       cfg.TrustProxyHeaders,
		UserUploadsPerMinute:                    cfg.UserUploadsPerMinute,
		UserUploadBurst:                         cfg.UserUploadBurst,
		UserConversionsPerMinute:                cfg.UserConversionsPerMinute,
		UserConversionBurst:                     cfg.UserConversionBurst,
		GuestCumulativeQuotaBytes:               cfg.GuestCumulativeQuotaBytes,
		RegisteredCumulativeQuotaBytes:          cfg.RegisteredCumulativeQuotaBytes,
		RequireVerifiedEmailForSensitiveActions: cfg.RequireVerifiedEmailForSensitiveActions,
		ArtifactTTLHours:                        cfg.ArtifactTTLHours,
		ArtifactTTLByFamily: map[string]int{
			"pdf":      cfg.ArtifactTTLByFamily[domain.FamilyPDF],
			"image":    cfg.ArtifactTTLByFamily[domain.FamilyImage],
			"document": cfg.ArtifactTTLByFamily[domain.FamilyDocument],
			"audio":    cfg.ArtifactTTLByFamily[domain.FamilyAudio],
			"video":    cfg.ArtifactTTLByFamily[domain.FamilyVideo],
		},
		QueueMode:         queueMode,
		WorkerConcurrency: effectiveWorkerConcurrency,
		RedisURL:          cfg.RedisURL,
	})

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      otelhttp.NewHandler(router, "reform-api"),
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
	return workers.BuildDefaultRegistry()
}
