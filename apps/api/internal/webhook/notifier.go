package webhook

import (
	"context"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type Notifier struct {
	queue    queue.JobQueue
	webhooks repository.WebhookRepository
	files    repository.FileRepository
	logger   zerolog.Logger
}

func NewNotifier(q queue.JobQueue, webhooks repository.WebhookRepository, files repository.FileRepository, logger zerolog.Logger) *Notifier {
	return &Notifier{
		queue:    q,
		webhooks: webhooks,
		files:    files,
		logger:   logger.With().Str("component", "webhook_notifier").Logger(),
	}
}

func (n *Notifier) NotifyJobCompleted(ctx context.Context, job *domain.Job) error {
	return n.notify(ctx, domain.WebhookEventJobCompleted, job)
}

func (n *Notifier) NotifyJobFailed(ctx context.Context, job *domain.Job) error {
	return n.notify(ctx, domain.WebhookEventJobFailed, job)
}

func (n *Notifier) notify(ctx context.Context, eventType domain.WebhookEventType, job *domain.Job) error {
	if job == nil {
		return nil
	}

	webhooks, err := n.webhooks.ListEnabledByEventType(ctx, eventType)
	if err != nil {
		n.logger.Warn().Err(err).Str("event_type", string(eventType)).Msg("failed to load webhooks")
		return err
	}
	if len(webhooks) == 0 {
		return nil
	}

	var originalName string
	if file, fileErr := n.files.GetByID(ctx, job.FileID); fileErr == nil && file != nil {
		originalName = file.OriginalName
	}

	eventID := uuid.NewString()
	payload := map[string]interface{}{
		"id":         eventID,
		"type":       string(eventType),
		"occurredAt": time.Now().UTC().Format(time.RFC3339),
		"job": map[string]interface{}{
			"id":           job.ID.String(),
			"fileId":       job.FileID.String(),
			"fileName":     originalName,
			"capabilityId": job.CapabilityID,
			"outputFormat": job.OutputFormat,
			"status":       job.Status,
			"progress":     job.Progress,
			"artifactId":   optionalUUIDString(job.ArtifactID),
			"error":        optionalString(job.Error),
			"startedAt":    optionalTimeString(job.StartedAt),
			"completedAt":  optionalTimeString(job.CompletedAt),
			"createdAt":    job.CreatedAt.Format(time.RFC3339),
			"userId":       optionalUUIDString(job.UserID),
		},
	}

	for _, webhook := range webhooks {
		err := n.queue.EnqueueWebhook(ctx, queue.WebhookTaskPayload{
			WebhookID: webhook.ID.String(),
			URL:       webhook.URL,
			Secret:    webhook.Secret,
			EventID:   eventID,
			EventType: string(eventType),
			Payload:   payload,
		}, queue.TaskOptions{MaxRetries: 3, Timeout: 15 * time.Second})
		if err != nil {
			n.logger.Warn().Err(err).Str("webhook_id", webhook.ID.String()).Str("event_type", string(eventType)).Msg("failed to enqueue webhook")
		}
	}

	return nil
}

func optionalUUIDString(value *uuid.UUID) interface{} {
	if value == nil {
		return nil
	}
	return value.String()
}

func optionalString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func optionalTimeString(value *time.Time) interface{} {
	if value == nil {
		return nil
	}
	return value.Format(time.RFC3339)
}
