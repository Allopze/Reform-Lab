package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type WebhookHandler struct {
	Webhooks repository.WebhookRepository
}

type webhookRequest struct {
	URL        string   `json:"url"`
	Secret     string   `json:"secret"`
	EventTypes []string `json:"eventTypes"`
	Enabled    *bool    `json:"enabled"`
}

type webhookResponse struct {
	ID              string                    `json:"id"`
	URL             string                    `json:"url"`
	EventTypes      []string                  `json:"eventTypes"`
	Enabled         bool                      `json:"enabled"`
	HasSecret       bool                      `json:"hasSecret"`
	LastDeliveredAt *string                   `json:"lastDeliveredAt,omitempty"`
	LastError       *string                   `json:"lastError,omitempty"`
	Deliveries      []webhookDeliveryResponse `json:"deliveries,omitempty"`
	CreatedAt       string                    `json:"createdAt"`
	UpdatedAt       string                    `json:"updatedAt"`
}

type webhookDeliveryResponse struct {
	ID          string  `json:"id"`
	EventID     string  `json:"eventId"`
	EventType   string  `json:"eventType"`
	AttemptedAt string  `json:"attemptedAt"`
	DeliveredAt *string `json:"deliveredAt,omitempty"`
	StatusCode  *int    `json:"statusCode,omitempty"`
	Error       *string `json:"error,omitempty"`
}

const webhookDeliveryHistoryLimit = 5

func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	webhooks, err := h.Webhooks.ListAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list webhooks")
		return
	}

	result := make([]webhookResponse, 0, len(webhooks))
	for _, webhook := range webhooks {
		deliveries, deliveryErr := h.Webhooks.ListDeliveries(r.Context(), webhook.ID, webhookDeliveryHistoryLimit)
		if deliveryErr != nil {
			respondError(w, http.StatusInternalServerError, "failed to list webhooks")
			return
		}
		result = append(result, buildWebhookResponse(webhook, deliveries))
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	eventTypes, err := parseWebhookEventTypes(req.EventTypes)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateWebhookURL(req.URL); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now().UTC()
	webhook := &domain.WebhookSubscription{
		ID:         uuid.New(),
		URL:        req.URL,
		Secret:     req.Secret,
		EventTypes: eventTypes,
		Enabled:    enabled,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := h.Webhooks.Create(r.Context(), webhook); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create webhook")
		return
	}

	respondJSON(w, http.StatusCreated, buildWebhookResponse(*webhook, nil))
}

func (h *WebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	webhookID, err := uuid.Parse(chi.URLParam(r, "webhookId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	current, err := h.Webhooks.GetByID(r.Context(), webhookID)
	if err != nil {
		respondError(w, http.StatusNotFound, "webhook not found")
		return
	}

	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	eventTypes, err := parseWebhookEventTypes(req.EventTypes)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateWebhookURL(req.URL); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	current.URL = req.URL
	current.EventTypes = eventTypes
	if req.Secret != "" {
		current.Secret = req.Secret
	}
	if req.Enabled != nil {
		current.Enabled = *req.Enabled
	}
	current.UpdatedAt = time.Now().UTC()

	if err := h.Webhooks.Update(r.Context(), current); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update webhook")
		return
	}

	deliveries, err := h.Webhooks.ListDeliveries(r.Context(), current.ID, webhookDeliveryHistoryLimit)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update webhook")
		return
	}

	respondJSON(w, http.StatusOK, buildWebhookResponse(*current, deliveries))
}

func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	webhookID, err := uuid.Parse(chi.URLParam(r, "webhookId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid webhook ID")
		return
	}

	if err := h.Webhooks.DeleteByID(r.Context(), webhookID); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete webhook")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func buildWebhookResponse(webhook domain.WebhookSubscription, deliveries []domain.WebhookDelivery) webhookResponse {
	eventTypes := make([]string, 0, len(webhook.EventTypes))
	for _, eventType := range webhook.EventTypes {
		eventTypes = append(eventTypes, string(eventType))
	}

	deliveryResponses := make([]webhookDeliveryResponse, 0, len(deliveries))
	for _, delivery := range deliveries {
		deliveryResponses = append(deliveryResponses, webhookDeliveryResponse{
			ID:          delivery.ID.String(),
			EventID:     delivery.EventID,
			EventType:   string(delivery.EventType),
			AttemptedAt: delivery.AttemptedAt.Format(time.RFC3339),
			DeliveredAt: stringTimePtr(delivery.DeliveredAt),
			StatusCode:  delivery.StatusCode,
			Error:       delivery.Error,
		})
	}

	return webhookResponse{
		ID:              webhook.ID.String(),
		URL:             webhook.URL,
		EventTypes:      eventTypes,
		Enabled:         webhook.Enabled,
		HasSecret:       webhook.Secret != "",
		LastDeliveredAt: stringTimePtr(webhook.LastDeliveredAt),
		LastError:       webhook.LastError,
		Deliveries:      deliveryResponses,
		CreatedAt:       webhook.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       webhook.UpdatedAt.Format(time.RFC3339),
	}
}

func parseWebhookEventTypes(raw []string) ([]domain.WebhookEventType, error) {
	if len(raw) == 0 {
		return nil, errString("at least one webhook event type is required")
	}

	allowed := make(map[string]domain.WebhookEventType)
	for _, eventType := range domain.SupportedWebhookEventTypes() {
		allowed[string(eventType)] = eventType
	}

	seen := make(map[domain.WebhookEventType]struct{})
	eventTypes := make([]domain.WebhookEventType, 0, len(raw))
	for _, value := range raw {
		eventType, ok := allowed[value]
		if !ok {
			return nil, errString("unsupported webhook event type")
		}
		if _, exists := seen[eventType]; exists {
			continue
		}
		seen[eventType] = struct{}{}
		eventTypes = append(eventTypes, eventType)
	}
	return eventTypes, nil
}

func validateWebhookURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errString("invalid webhook URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errString("webhook URL must use http or https")
	}
	if parsed.Host == "" {
		return errString("invalid webhook URL")
	}
	return nil
}

type errString string

func (e errString) Error() string { return string(e) }

func stringTimePtr(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}
