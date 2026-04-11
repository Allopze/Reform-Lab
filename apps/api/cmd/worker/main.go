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
	"github.com/allopze/reform-lab/apps/api/internal/email"
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
	siteSettingRepo := repository.NewSiteSettingRepository(db)
	emailTemplateRepo := repository.NewEmailTemplateRepository(db)

	// Email
	emailSvc := email.NewService(cfg, siteSettingRepo, emailTemplateRepo, logger)
	emailWorker := &workers.EmailHandler{
		Email:  emailSvc,
		Logger: logger.With().Str("component", "email_worker").Logger(),
	}

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
	registry.Register("pdf-compress", &pdf.CompressEngine{})
	registry.Register("pdf-to-html-preview", &pdf.ToHTMLEngine{})
	registry.Register("pdf-ocr-to-txt", &pdf.OCRToTextEngine{})
	registry.Register("pdf-ocr-to-json", &pdf.OCRToJSONEngine{})
	registry.Register("pdf-ocr-searchable-pdf", &pdf.OCRToSearchablePDFEngine{})

	imgConvert := &workerImage.ConvertEngine{}
	registry.Register("image-to-png", imgConvert)
	registry.Register("image-to-jpg", imgConvert)
	registry.Register("image-to-webp", &workerImage.ToWebPEngine{})
	registry.Register("image-to-avif", &workerImage.ToAVIFEngine{})
	registry.Register("image-to-pdf", &workerImage.ToPDFEngine{})
	registry.Register("image-heic-to-jpg", &workerImage.HEIFConvertEngine{})
	registry.Register("image-heic-to-png", &workerImage.HEIFConvertEngine{})
	registry.Register("image-heic-to-webp", &workerImage.HEIFConvertEngine{})
	registry.Register("image-svg-to-png", &workerImage.SVGConvertEngine{})
	registry.Register("image-svg-to-webp", &workerImage.SVGConvertEngine{})
	registry.Register("image-svg-to-pdf", &workerImage.SVGConvertEngine{})
	registry.Register("image-compress-jpg", &workerImage.CompressEngine{})
	registry.Register("image-compress-png", &workerImage.CompressEngine{})
	registry.Register("image-thumbnail-jpg", &workerImage.ThumbnailEngine{})
	registry.Register("image-thumbnail-png", &workerImage.ThumbnailEngine{})
	registry.Register("image-ocr-to-txt", &workerImage.OCRToTextEngine{})
	registry.Register("image-ocr-to-json", &workerImage.OCRToJSONEngine{})
	smallWebVariant := workerImage.NewWebVariantEngine(640)
	largeWebVariant := workerImage.NewWebVariantEngine(1600)
	registry.Register("image-web-jpg-640", smallWebVariant)
	registry.Register("image-web-webp-640", smallWebVariant)
	registry.Register("image-web-avif-640", smallWebVariant)
	registry.Register("image-web-jpg-1600", largeWebVariant)
	registry.Register("image-web-webp-1600", largeWebVariant)
	registry.Register("image-web-avif-1600", largeWebVariant)

	registry.Register("doc-to-pdf", &document.ToPDFEngine{})
	registry.Register("doc-to-txt", &document.ToTxtEngine{})
	registry.Register("doc-to-docx", &document.ToDocxEngine{})
	registry.Register("doc-to-html", &document.ToHTMLEngine{})
	registry.Register("docx-to-markdown", &document.DOCXToMarkdownEngine{})
	registry.Register("txt-to-pdf", &document.ToPDFEngine{})
	registry.Register("html-to-pdf", &document.ToPDFEngine{})
	registry.Register("html-to-txt", &document.HTMLToTextEngine{})
	registry.Register("markdown-to-html", &document.MarkdownToHTMLEngine{})
	registry.Register("markdown-to-pdf", &document.MarkdownToPDFEngine{})
	registry.Register("markdown-to-docx", &document.MarkdownToDocxEngine{})
	registry.Register("presentation-to-pdf", &document.ToPDFEngine{})
	registry.Register("presentation-to-jpg", &document.PresentationToImagesEngine{})
	registry.Register("presentation-to-png", &document.PresentationToImagesEngine{})
	registry.Register("spreadsheet-to-pdf", &document.ToPDFEngine{})
	registry.Register("spreadsheet-to-csv", &document.ToCSVEngine{})
	registry.Register("spreadsheet-to-xlsx", &document.ToXLSXEngine{})
	registry.Register("spreadsheet-to-html", &document.ToHTMLEngine{})

	audioConvert := &audio.ConvertEngine{}
	registry.Register("audio-to-mp3", audioConvert)
	registry.Register("audio-to-wav", audioConvert)
	registry.Register("audio-to-ogg", audioConvert)
	registry.Register("audio-to-aac", audioConvert)
	registry.Register("audio-to-m4a", audioConvert)
	registry.Register("audio-to-flac", audioConvert)
	registry.Register("audio-to-opus", audioConvert)
	registry.Register("video-to-mp3", audioConvert)
	registry.Register("video-to-wav", audioConvert)
	registry.Register("video-to-aac", audioConvert)
	registry.Register("video-to-m4a", audioConvert)
	registry.Register("video-to-flac", audioConvert)
	registry.Register("video-to-opus", audioConvert)
	waveform := &audio.WaveformEngine{}
	registry.Register("audio-waveform-png", waveform)
	registry.Register("video-waveform-png", waveform)

	videoConvert := &video.ConvertEngine{}
	registry.Register("video-to-mp4", videoConvert)
	registry.Register("video-to-webm", videoConvert)
	registry.Register("video-to-gif", &video.ToGIFEngine{})
	registry.Register("video-to-thumbnails", &video.ThumbnailZipEngine{})
	registry.Register("video-contact-sheet", &video.ContactSheetEngine{})
	registry.Register("video-preview-mp4", &video.PreviewClipEngine{})
	registry.Register("video-preview-webm", &video.PreviewClipEngine{})

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
		Concurrency: cfg.WorkerConcurrency,
		Queues:      map[string]int{"default": 1},
		Logger:      &asynqLogger{logger: logger},
	})

	// Map all conversion task types to the generic handler via Asynq adapter.
	mux := asynq.NewServeMux()
	for _, capID := range []string{
		"pdf-to-jpg", "pdf-to-png", "pdf-to-txt", "pdf-to-docx", "pdf-compress", "pdf-to-html-preview", "pdf-ocr-to-txt", "pdf-ocr-to-json", "pdf-ocr-searchable-pdf",
		"image-to-png", "image-to-jpg", "image-to-webp", "image-to-avif", "image-to-pdf", "image-heic-to-jpg", "image-heic-to-png", "image-heic-to-webp", "image-svg-to-png", "image-svg-to-webp", "image-svg-to-pdf", "image-compress-jpg", "image-compress-png", "image-thumbnail-jpg", "image-thumbnail-png", "image-ocr-to-txt", "image-ocr-to-json", "image-web-jpg-640", "image-web-webp-640", "image-web-avif-640", "image-web-jpg-1600", "image-web-webp-1600", "image-web-avif-1600",
		"doc-to-pdf", "doc-to-txt", "doc-to-docx", "doc-to-html", "docx-to-markdown", "txt-to-pdf", "html-to-pdf", "html-to-txt", "markdown-to-html", "markdown-to-pdf", "markdown-to-docx", "presentation-to-pdf", "presentation-to-jpg", "presentation-to-png", "spreadsheet-to-pdf", "spreadsheet-to-csv", "spreadsheet-to-xlsx", "spreadsheet-to-html",
		"audio-to-mp3", "audio-to-wav", "audio-to-ogg", "audio-to-aac", "audio-to-m4a", "audio-to-flac", "audio-to-opus", "audio-waveform-png", "video-to-mp3", "video-to-wav", "video-to-aac", "video-to-m4a", "video-to-flac", "video-to-opus",
		"video-to-mp4", "video-to-webm", "video-to-gif", "video-to-thumbnails", "video-contact-sheet", "video-preview-mp4", "video-preview-webm", "video-waveform-png",
	} {
		taskType := "conversion:" + capID
		mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
			return handler.ProcessPayload(ctx, t.Type(), t.Payload())
		})
	}

	// Register email task handler.
	mux.HandleFunc(queue.EmailTaskType, func(ctx context.Context, t *asynq.Task) error {
		return emailWorker.ProcessPayload(ctx, t.Type(), t.Payload())
	})

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logger.Info().Msg("shutting down worker")
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
