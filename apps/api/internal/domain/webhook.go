package domain

import (
	"time"

	"github.com/google/uuid"
)

type WebhookEventType string

const (
	WebhookEventJobCompleted WebhookEventType = "job.completed"
	WebhookEventJobFailed    WebhookEventType = "job.failed"
)

func SupportedWebhookEventTypes() []WebhookEventType {
	return []WebhookEventType{
		WebhookEventJobCompleted,
		WebhookEventJobFailed,
	}
}

type WebhookSubscription struct {
	ID              uuid.UUID          `json:"id"`
	URL             string             `json:"url"`
	Secret          string             `json:"-"`
	EventTypes      []WebhookEventType `json:"eventTypes"`
	Enabled         bool               `json:"enabled"`
	LastDeliveredAt *time.Time         `json:"lastDeliveredAt,omitempty"`
	LastError       *string            `json:"lastError,omitempty"`
	CreatedAt       time.Time          `json:"createdAt"`
	UpdatedAt       time.Time          `json:"updatedAt"`
}

type WebhookDelivery struct {
	ID          uuid.UUID        `json:"id"`
	WebhookID   uuid.UUID        `json:"webhookId"`
	EventID     string           `json:"eventId"`
	EventType   WebhookEventType `json:"eventType"`
	AttemptedAt time.Time        `json:"attemptedAt"`
	DeliveredAt *time.Time       `json:"deliveredAt,omitempty"`
	StatusCode  *int             `json:"statusCode,omitempty"`
	Error       *string          `json:"error,omitempty"`
}
