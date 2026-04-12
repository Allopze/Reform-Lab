package workers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Handler processes conversion tasks by delegating to the appropriate engine.
type Handler struct {
	Registry            *Registry
	Store               storage.Store
	Artifacts           repository.ArtifactRepository
	Audit               repository.AuditRepository
	Orch                *orchestrator.Service
	Logger              zerolog.Logger
	Metrics             *observability.Metrics
	ArtifactTTL         time.Duration
	ArtifactTTLByFamily map[domain.FormatFamily]time.Duration
}

// ProcessPayload processes raw task payload bytes. Used by both in-process and Asynq adapters.
func (h *Handler) ProcessPayload(ctx context.Context, _ string, data []byte) error {
	var payload queue.TaskPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	ctx, span := otel.Tracer("reform-worker").Start(ctx, "conversion.process")
	span.SetAttributes(
		attribute.String("job.id", payload.JobID),
		attribute.String("capability.id", payload.CapabilityID),
		attribute.String("file.id", payload.FileID),
	)
	defer span.End()

	jobID, err := uuid.Parse(payload.JobID)
	if err != nil {
		return fmt.Errorf("parse job ID: %w", err)
	}
	var userID *uuid.UUID
	if payload.UserID != "" {
		parsedUserID, parseErr := uuid.Parse(payload.UserID)
		if parseErr != nil {
			return fmt.Errorf("parse user ID: %w", parseErr)
		}
		userID = &parsedUserID
	}
	fileID, err := uuid.Parse(payload.FileID)
	if err != nil {
		return fmt.Errorf("parse file ID: %w", err)
	}

	logger := h.Logger.With().
		Str("job_id", payload.JobID).
		Str("capability_id", payload.CapabilityID).
		Logger()

	// Mark job as running.
	h.Metrics.ActiveJobs.WithLabelValues("running").Inc()
	defer h.Metrics.ActiveJobs.WithLabelValues("running").Dec()
	if err := h.Orch.MarkRunning(ctx, jobID); err != nil {
		logger.Error().Err(err).Msg("failed to mark job running")
		return err
	}

	start := time.Now()
	capability := capabilities.ByID(payload.CapabilityID)
	if capability == nil {
		return h.fail(ctx, jobID, logger, "resolve capability", fmt.Errorf("capability definition not found"))
	}
	if !capabilities.DefaultFlags.Allows(*capability) {
		return h.fail(ctx, jobID, logger, "feature flag", fmt.Errorf("capability disabled by feature flag"))
	}

	// Create temp dir for this job.
	_ = h.Orch.UpdateProgress(ctx, jobID, 20) // preparing workspace
	tempDir, err := h.Store.CreateTempDir(ctx, payload.JobID)
	if err != nil {
		return h.fail(ctx, jobID, logger, "create temp dir", err)
	}
	defer h.Store.CleanupTemp(ctx, payload.JobID)

	// Find the engine.
	_ = h.Orch.UpdateProgress(ctx, jobID, 30) // resolving engine
	engine, err := h.Registry.Get(payload.CapabilityID)
	if err != nil {
		return h.fail(ctx, jobID, logger, "find engine", err)
	}

	// Execute conversion.
	execCtx, stopWatching := h.newExecutionContext(ctx, jobID, logger)
	defer stopWatching()
	_ = h.Orch.UpdateProgress(ctx, jobID, 40) // converting
	outputPath, err := engine.Execute(execCtx, payload.InputPath, tempDir, payload.OutputFormat)
	if err != nil {
		if errors.Is(err, context.Canceled) && h.isCancelled(ctx, jobID) {
			logger.Info().Msg("job cancelled during engine execution")
			return nil
		}
		return h.fail(ctx, jobID, logger, "execute conversion", err)
	}

	artifactFormat := outputArtifactFormat(outputPath, payload.OutputFormat)

	// Validate output exists, has content, and matches the expected artifact format.
	_ = h.Orch.UpdateProgress(ctx, jobID, 70) // validating output
	info, err := validateOutputArtifact(outputPath, artifactFormat)
	if err != nil {
		return h.fail(ctx, jobID, logger, "validate output", err)
	}
	if h.isCancelled(ctx, jobID) {
		logger.Info().Msg("job cancelled before artifact persistence")
		return nil
	}

	// Persist artifact.
	_ = h.Orch.UpdateProgress(ctx, jobID, 80) // saving artifact
	artifactID := uuid.New()
	outputFile, err := os.Open(outputPath)
	if err != nil {
		return h.fail(ctx, jobID, logger, "open output", err)
	}
	defer outputFile.Close()

	fileName := outputArtifactFileName(outputPath, artifactFormat)
	storagePath, err := h.Store.SaveArtifact(ctx, artifactID.String(), fileName, outputFile)
	if err != nil {
		return h.fail(ctx, jobID, logger, "save artifact", err)
	}

	now := time.Now().UTC()
	artifact := domain.Artifact{
		ID:          artifactID,
		UserID:      userID,
		JobID:       jobID,
		FileID:      fileID,
		FileName:    fileName,
		MIMEType:    mimeForFormat(artifactFormat),
		Size:        info.Size(),
		StoragePath: storagePath,
		CreatedAt:   now,
		ExpiresAt:   now.Add(h.artifactTTL(artifactFormat)),
	}

	if err := h.Artifacts.Create(ctx, &artifact); err != nil {
		_ = os.RemoveAll(filepath.Dir(storagePath))
		return h.fail(ctx, jobID, logger, "persist artifact record", err)
	}
	if h.isCancelled(ctx, jobID) {
		logger.Info().Str("artifact_id", artifactID.String()).Msg("job cancelled after artifact persistence; cleaning artifact")
		_ = os.RemoveAll(filepath.Dir(storagePath))
		_ = h.Artifacts.DeleteByID(ctx, artifactID)
		return nil
	}

	_ = h.Audit.Create(ctx, &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: domain.AuditArtifactCreated,
		FileID:    &fileID,
		JobID:     &jobID,
		Details: map[string]interface{}{
			"artifactId": artifactID.String(),
			"fileName":   fileName,
			"size":       info.Size(),
		},
		CreatedAt: now,
	})

	// Mark succeeded.
	_ = h.Orch.UpdateProgress(ctx, jobID, 95) // finalizing
	if err := h.Orch.MarkSucceeded(ctx, jobID, artifactID); err != nil {
		logger.Error().Err(err).Msg("failed to mark job succeeded")
		return err
	}

	duration := time.Since(start).Seconds()
	h.Metrics.JobsTotal.WithLabelValues(payload.CapabilityID, "succeeded").Inc()
	h.Metrics.JobDuration.WithLabelValues(payload.CapabilityID).Observe(duration)
	h.Metrics.ArtifactsTotal.Inc()

	logger.Info().Float64("duration_sec", duration).Msg("conversion succeeded")
	return nil
}

func (h *Handler) fail(ctx context.Context, jobID uuid.UUID, logger zerolog.Logger, step string, err error) error {
	msg := classifyError(step, err)
	logger.Error().Err(err).Str("step", step).Msg("conversion failed")
	_ = h.Orch.MarkFailed(ctx, jobID, msg)
	h.Metrics.JobsTotal.WithLabelValues("", "failed").Inc()
	h.Metrics.ErrorsTotal.WithLabelValues(step).Inc()
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		span.SetStatus(codes.Error, msg)
		span.RecordError(err)
	}
	return fmt.Errorf("%s: %w", step, err)
}

// classifyError produces a user-friendly error message from a worker step and Go error.
func classifyError(step string, err error) string {
	errText := err.Error()
	switch {
	case step == "feature flag":
		return "Capacidad deshabilitada temporalmente. Intenta más tarde."
	case step == "find engine":
		return "Motor de conversión no disponible. Intenta más tarde."
	case step == "validate output":
		return "La conversión no produjo un resultado válido."
	case step == "create temp dir" || step == "save artifact":
		return "Error de almacenamiento interno. Intenta más tarde."
	case containsAny(errText, "signal: killed", "context deadline exceeded", "timeout"):
		return "La conversión excedió el tiempo máximo permitido."
	case containsAny(errText, "exit status"):
		return "El motor de conversión no pudo procesar este archivo."
	default:
		return fmt.Sprintf("%s: %v", step, err)
	}
}

func (h *Handler) isCancelled(ctx context.Context, jobID uuid.UUID) bool {
	job, err := h.Orch.GetJob(ctx, jobID)
	if err != nil {
		return false
	}
	return job.Status == domain.JobCancelled
}

func (h *Handler) newExecutionContext(parent context.Context, jobID uuid.UUID, logger zerolog.Logger) (context.Context, context.CancelFunc) {
	execCtx, cancel := context.WithCancel(parent)
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-execCtx.Done():
				return
			case <-ticker.C:
				if h.isCancelled(parent, jobID) {
					logger.Info().Msg("job cancellation detected; stopping execution context")
					cancel()
					return
				}
			}
		}
	}()
	return execCtx, cancel
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

func (h *Handler) artifactTTL(outputFormat string) time.Duration {
	if ttl, ok := h.ArtifactTTLByFamily[familyForOutputFormat(outputFormat)]; ok && ttl > 0 {
		return ttl
	}
	if h.ArtifactTTL > 0 {
		return h.ArtifactTTL
	}
	return 24 * time.Hour
}

func familyForOutputFormat(format string) domain.FormatFamily {
	switch format {
	case "pdf":
		return domain.FamilyPDF
	case "jpg", "png", "webp", "gif", "zip", "avif":
		return domain.FamilyImage
	case "docx", "txt", "html", "md", "json", "csv", "xlsx":
		return domain.FamilyDocument
	case "mp3", "wav", "ogg", "aac", "flac", "m4a", "opus":
		return domain.FamilyAudio
	case "mp4", "webm":
		return domain.FamilyVideo
	default:
		return domain.FamilyDocument
	}
}

func mimeForFormat(format string) string {
	m := map[string]string{
		"pdf":  "application/pdf",
		"docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"json": "application/json",
		"md":   "text/markdown",
		"txt":  "text/plain",
		"csv":  "text/csv",
		"html": "text/html",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"jpg":  "image/jpeg",
		"png":  "image/png",
		"webp": "image/webp",
		"avif": "image/avif",
		"gif":  "image/gif",
		"mp3":  "audio/mpeg",
		"wav":  "audio/wav",
		"ogg":  "audio/ogg",
		"aac":  "audio/aac",
		"m4a":  "audio/mp4",
		"flac": "audio/flac",
		"opus": "audio/opus",
		"mp4":  "video/mp4",
		"webm": "video/webm",
		"zip":  "application/zip",
	}
	if v, ok := m[format]; ok {
		return v
	}
	return "application/octet-stream"
}

func outputArtifactFormat(outputPath, fallback string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(outputPath)), ".")
	if ext == "" {
		return fallback
	}
	return ext
}

func outputArtifactFileName(outputPath, fallbackFormat string) string {
	name := strings.TrimSpace(filepath.Base(outputPath))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return fmt.Sprintf("converted.%s", fallbackFormat)
	}
	return name
}
