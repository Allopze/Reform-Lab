package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/allopze/reform-lab/apps/api/config"
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

	shutdownTracer, err := observability.InitTracer(context.Background(), "reform-worker", "1.0.0")
	if err != nil {
		logger.Fatal().Err(err).Msg("init tracer")
	}
	defer shutdownTracer(context.Background())

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
	secretKeeper, err := security.NewSecretKeeper(cfg.SecretEncryptionKey)
	if err != nil {
		logger.Fatal().Err(err).Msg("init secret keeper")
	}

	// Repositories
	jobRepo := repository.NewJobRepository(db)
	artifactRepo := repository.NewArtifactRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	siteSettingRepo := repository.NewSiteSettingRepository(db)
	emailTemplateRepo := repository.NewEmailTemplateRepository(db)
	webhookRepo := repository.NewWebhookRepository(db, repository.WithSecretKeeper(secretKeeper))
	workerStatusRepo := repository.NewWorkerStatusRepository(db)
	runtimeControlRepo := repository.NewRuntimeControlRepository(db)

	// Email
	emailSvc := email.NewService(cfg, siteSettingRepo, emailTemplateRepo, logger, email.WithSecretKeeper(secretKeeper))
	emailWorker := &workers.EmailHandler{
		Email:  emailSvc,
		Logger: logger.With().Str("component", "email_worker").Logger(),
	}
	webhookWorker := &workers.WebhookHandler{
		Webhooks: webhookRepo,
		Logger:   logger.With().Str("component", "webhook_worker").Logger(),
	}

	// Queue (for orchestrator to update jobs)
	q, err := queue.NewAsynqQueue(cfg.RedisURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("init redis queue")
	}
	defer q.Close()

	webhookNotifier := webhookpkg.NewNotifier(q, webhookRepo, repository.NewFileRepository(db), logger)
	orch := orchestrator.NewService(jobRepo, auditRepo, q, orchestrator.WithNotifier(webhookNotifier), orchestrator.WithRuntimeControls(runtimeControlRepo))

	// Register conversion engines.
	registry := workers.BuildDefaultRegistry()

	hostname, _ := os.Hostname()
	workerID := hostname + "-" + strconv.Itoa(os.Getpid())
	handler := &workers.Handler{
		Registry:     registry,
		Store:        store,
		Artifacts:    artifactRepo,
		Audit:        auditRepo,
		Orch:         orch,
		WorkerStatus: workerStatusRepo,
		WorkerID:     workerID,
		RuntimeMode:  "standalone",
		QueueMode:    "redis",
		Logger:       logger,
		Metrics:      metrics,
		ArtifactTTL:  time.Duration(cfg.ArtifactTTLHours) * time.Hour,
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
		Concurrency: cfg.WorkerConcurrency,
		Queues:      map[string]int{"default": 1},
		Logger:      &asynqLogger{logger: logger},
	})

	// Map all conversion task types to the generic handler via Asynq adapter.
	mux := asynq.NewServeMux()
	for _, capID := range workers.DefaultCapabilityIDs() {
		taskType := "conversion:" + capID
		mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
			return handler.ProcessPayload(ctx, t.Type(), t.Payload())
		})
	}

	// Register email task handler.
	mux.HandleFunc(queue.EmailTaskType, func(ctx context.Context, t *asynq.Task) error {
		return emailWorker.ProcessPayload(ctx, t.Type(), t.Payload())
	})
	mux.HandleFunc(queue.WebhookTaskType, func(ctx context.Context, t *asynq.Task) error {
		return webhookWorker.ProcessPayload(ctx, t.Type(), t.Payload())
	})

	// Graceful shutdown
	heartbeatCtx, stopHeartbeat := context.WithCancel(context.Background())
	defer stopHeartbeat()
	workers.StartHeartbeatLoop(heartbeatCtx, workerStatusRepo, workerID, "standalone", "redis", 10*time.Second)
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info().Msg("shutting down worker")
		stopHeartbeat()
		srv.Shutdown()
	}()

	logger.Info().Int("concurrency", cfg.WorkerConcurrency).Msg("starting worker (Redis mode)")
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
