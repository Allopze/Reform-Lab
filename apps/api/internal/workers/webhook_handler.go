package workers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type WebhookHandler struct {
	Webhooks repository.WebhookRepository
	Client   *http.Client
	Logger   zerolog.Logger
}

func (h *WebhookHandler) ProcessPayload(ctx context.Context, _ string, data []byte) error {
	var payload queue.WebhookTaskPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("unmarshal webhook payload: %w", err)
	}

	webhookID, err := uuid.Parse(payload.WebhookID)
	if err != nil {
		return fmt.Errorf("parse webhook ID: %w", err)
	}
	attemptedAt := time.Now().UTC()

	body, err := json.Marshal(payload.Payload)
	if err != nil {
		return fmt.Errorf("marshal webhook body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, payload.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-ReformLab-Event", payload.EventType)
	req.Header.Set("X-ReformLab-Event-Id", payload.EventID)
	if payload.Secret != "" {
		req.Header.Set("X-ReformLab-Signature", signWebhookPayload(payload.Secret, body))
	}

	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		message := err.Error()
		h.recordDeliveryResult(ctx, webhookID, payload, attemptedAt, nil, nil, &message)
		return fmt.Errorf("deliver webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodySnippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		message := fmt.Sprintf("unexpected status %d: %s", resp.StatusCode, string(bodySnippet))
		statusCode := resp.StatusCode
		h.recordDeliveryResult(ctx, webhookID, payload, attemptedAt, nil, &statusCode, &message)
		return fmt.Errorf("deliver webhook: %s", message)
	}

	deliveredAt := time.Now().UTC()
	statusCode := resp.StatusCode
	h.recordDeliveryResult(ctx, webhookID, payload, attemptedAt, &deliveredAt, &statusCode, nil)
	h.Logger.Info().Str("webhook_id", payload.WebhookID).Str("event_type", payload.EventType).Msg("webhook delivered")
	return nil
}

func (h *WebhookHandler) recordDeliveryResult(
	ctx context.Context,
	webhookID uuid.UUID,
	payload queue.WebhookTaskPayload,
	attemptedAt time.Time,
	deliveredAt *time.Time,
	statusCode *int,
	errorMessage *string,
) {
	delivery := &domain.WebhookDelivery{
		ID:          uuid.New(),
		WebhookID:   webhookID,
		EventID:     payload.EventID,
		EventType:   domain.WebhookEventType(payload.EventType),
		AttemptedAt: attemptedAt,
		DeliveredAt: deliveredAt,
		StatusCode:  statusCode,
		Error:       errorMessage,
	}
	if err := h.Webhooks.CreateDelivery(ctx, delivery); err != nil {
		h.Logger.Warn().Err(err).Str("webhook_id", payload.WebhookID).Msg("failed to persist webhook delivery")
	}
	if err := h.Webhooks.MarkDeliveryResult(ctx, webhookID, deliveredAt, errorMessage); err != nil {
		h.Logger.Warn().Err(err).Str("webhook_id", payload.WebhookID).Msg("failed to update webhook summary")
	}
}

func signWebhookPayload(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
