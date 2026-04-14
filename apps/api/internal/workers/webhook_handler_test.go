package workers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type webhookRepoSpy struct {
	createdDeliveries []*domain.WebhookDelivery
	markedID          uuid.UUID
	markedDeliveredAt *time.Time
	markedError       *string
}

func (r *webhookRepoSpy) Create(_ context.Context, _ *domain.WebhookSubscription) error {
	return nil
}

func (r *webhookRepoSpy) Update(_ context.Context, _ *domain.WebhookSubscription) error {
	return nil
}

func (r *webhookRepoSpy) GetByID(_ context.Context, _ uuid.UUID) (*domain.WebhookSubscription, error) {
	return nil, errors.New("not implemented")
}

func (r *webhookRepoSpy) ListAll(_ context.Context) ([]domain.WebhookSubscription, error) {
	return nil, nil
}

func (r *webhookRepoSpy) ListEnabledByEventType(_ context.Context, _ domain.WebhookEventType) ([]domain.WebhookSubscription, error) {
	return nil, nil
}

func (r *webhookRepoSpy) ListDeliveries(_ context.Context, _ uuid.UUID, _ int) ([]domain.WebhookDelivery, error) {
	return nil, nil
}

func (r *webhookRepoSpy) CreateDelivery(_ context.Context, delivery *domain.WebhookDelivery) error {
	r.createdDeliveries = append(r.createdDeliveries, delivery)
	return nil
}

func (r *webhookRepoSpy) DeleteByID(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (r *webhookRepoSpy) MarkDeliveryResult(_ context.Context, id uuid.UUID, deliveredAt *time.Time, lastError *string) error {
	r.markedID = id
	r.markedDeliveredAt = deliveredAt
	r.markedError = lastError
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestWebhookHandlerRecordsSuccess(t *testing.T) {
	repo := &webhookRepoSpy{}
	handler := &WebhookHandler{
		Webhooks: repo,
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get("X-ReformLab-Event") != "job.completed" {
				t.Fatalf("unexpected event header: %q", req.Header.Get("X-ReformLab-Event"))
			}
			return &http.Response{
				StatusCode: http.StatusAccepted,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
			}, nil
		})},
		Logger: zerolog.New(io.Discard),
	}

	webhookID := uuid.New()
	payload, err := json.Marshal(queue.WebhookTaskPayload{
		WebhookID: webhookID.String(),
		URL:       "https://example.com/webhook",
		EventID:   "evt-1",
		EventType: "job.completed",
		Payload:   map[string]interface{}{"id": "evt-1"},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if err := handler.ProcessPayload(context.Background(), queue.WebhookTaskType, payload); err != nil {
		t.Fatalf("process payload: %v", err)
	}
	if len(repo.createdDeliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(repo.createdDeliveries))
	}
	delivery := repo.createdDeliveries[0]
	if delivery.EventType != domain.WebhookEventJobCompleted {
		t.Fatalf("expected completed event, got %q", delivery.EventType)
	}
	if delivery.StatusCode == nil || *delivery.StatusCode != http.StatusAccepted {
		t.Fatalf("expected status code %d, got %#v", http.StatusAccepted, delivery.StatusCode)
	}
	if repo.markedDeliveredAt == nil {
		t.Fatal("expected successful delivery timestamp to be recorded")
	}
	if repo.markedError != nil {
		t.Fatalf("expected no summary error, got %v", *repo.markedError)
	}
}

func TestWebhookHandlerRecordsFailure(t *testing.T) {
	repo := &webhookRepoSpy{}
	handler := &WebhookHandler{
		Webhooks: repo,
		Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("gateway failure")),
				Header:     make(http.Header),
			}, nil
		})},
		Logger: zerolog.New(io.Discard),
	}

	webhookID := uuid.New()
	payload, err := json.Marshal(queue.WebhookTaskPayload{
		WebhookID: webhookID.String(),
		URL:       "https://example.com/webhook",
		EventID:   "evt-2",
		EventType: "job.failed",
		Payload:   map[string]interface{}{"id": "evt-2"},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	if err := handler.ProcessPayload(context.Background(), queue.WebhookTaskType, payload); err == nil {
		t.Fatal("expected delivery error")
	}
	if len(repo.createdDeliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(repo.createdDeliveries))
	}
	delivery := repo.createdDeliveries[0]
	if delivery.DeliveredAt != nil {
		t.Fatal("expected no delivered timestamp for failed webhook")
	}
	if delivery.Error == nil || !strings.Contains(*delivery.Error, "unexpected status 502") {
		t.Fatalf("expected stored failure message, got %#v", delivery.Error)
	}
	if repo.markedDeliveredAt != nil {
		t.Fatal("expected summary deliveredAt to remain nil")
	}
	if repo.markedError == nil || !strings.Contains(*repo.markedError, "unexpected status 502") {
		t.Fatalf("expected summary error to be recorded, got %#v", repo.markedError)
	}
}
