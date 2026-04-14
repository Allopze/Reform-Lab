package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/security"
	"github.com/google/uuid"
)

type WebhookRepository interface {
	Create(ctx context.Context, webhook *domain.WebhookSubscription) error
	Update(ctx context.Context, webhook *domain.WebhookSubscription) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.WebhookSubscription, error)
	ListAll(ctx context.Context) ([]domain.WebhookSubscription, error)
	ListEnabledByEventType(ctx context.Context, eventType domain.WebhookEventType) ([]domain.WebhookSubscription, error)
	ListDeliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]domain.WebhookDelivery, error)
	CreateDelivery(ctx context.Context, delivery *domain.WebhookDelivery) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
	MarkDeliveryResult(ctx context.Context, id uuid.UUID, deliveredAt *time.Time, lastError *string) error
}

type sqliteWebhookRepo struct {
	db      *sql.DB
	secrets *security.SecretKeeper
}

type WebhookRepoOption func(*sqliteWebhookRepo)

func WithSecretKeeper(keeper *security.SecretKeeper) WebhookRepoOption {
	return func(repo *sqliteWebhookRepo) {
		repo.secrets = keeper
	}
}

func NewWebhookRepository(db *sql.DB, opts ...WebhookRepoOption) WebhookRepository {
	repo := &sqliteWebhookRepo{db: db}
	for _, opt := range opts {
		if opt != nil {
			opt(repo)
		}
	}
	return repo
}

func (r *sqliteWebhookRepo) Create(ctx context.Context, webhook *domain.WebhookSubscription) error {
	eventTypes, err := marshalWebhookEventTypes(webhook.EventTypes)
	if err != nil {
		return err
	}
	secret, err := r.encryptSecret(webhook.Secret)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO webhooks (id, url, secret, event_types, enabled, last_delivered_at, last_error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		webhook.ID.String(),
		webhook.URL,
		secret,
		eventTypes,
		boolToInt(webhook.Enabled),
		fmtTimePtr(webhook.LastDeliveredAt),
		webhook.LastError,
		webhook.CreatedAt.Format(timeLayout),
		webhook.UpdatedAt.Format(timeLayout),
	)
	return err
}

func (r *sqliteWebhookRepo) Update(ctx context.Context, webhook *domain.WebhookSubscription) error {
	eventTypes, err := marshalWebhookEventTypes(webhook.EventTypes)
	if err != nil {
		return err
	}
	secret, err := r.encryptSecret(webhook.Secret)
	if err != nil {
		return err
	}

	_, err = r.db.ExecContext(ctx,
		`UPDATE webhooks
		 SET url = ?, secret = ?, event_types = ?, enabled = ?, updated_at = ?
		 WHERE id = ?`,
		webhook.URL,
		secret,
		eventTypes,
		boolToInt(webhook.Enabled),
		webhook.UpdatedAt.Format(timeLayout),
		webhook.ID.String(),
	)
	return err
}

func (r *sqliteWebhookRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.WebhookSubscription, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, url, secret, event_types, enabled, last_delivered_at, last_error, created_at, updated_at
		 FROM webhooks WHERE id = ?`,
		id.String(),
	)

	webhook, err := r.scanWebhook(row)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}

func (r *sqliteWebhookRepo) ListAll(ctx context.Context) ([]domain.WebhookSubscription, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, url, secret, event_types, enabled, last_delivered_at, last_error, created_at, updated_at
		 FROM webhooks ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []domain.WebhookSubscription
	for rows.Next() {
		webhook, scanErr := r.scanWebhook(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		webhooks = append(webhooks, *webhook)
	}
	return webhooks, rows.Err()
}

func (r *sqliteWebhookRepo) ListEnabledByEventType(ctx context.Context, eventType domain.WebhookEventType) ([]domain.WebhookSubscription, error) {
	all, err := r.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	filtered := make([]domain.WebhookSubscription, 0, len(all))
	for _, webhook := range all {
		if !webhook.Enabled {
			continue
		}
		for _, current := range webhook.EventTypes {
			if current == eventType {
				filtered = append(filtered, webhook)
				break
			}
		}
	}
	return filtered, nil
}

func (r *sqliteWebhookRepo) ListDeliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]domain.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, webhook_id, event_id, event_type, attempted_at, delivered_at, status_code, error
		 FROM webhook_deliveries
		 WHERE webhook_id = ?
		 ORDER BY attempted_at DESC
		 LIMIT ?`,
		webhookID.String(),
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deliveries := make([]domain.WebhookDelivery, 0, limit)
	for rows.Next() {
		delivery, scanErr := scanWebhookDelivery(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		deliveries = append(deliveries, *delivery)
	}
	return deliveries, rows.Err()
}

func (r *sqliteWebhookRepo) CreateDelivery(ctx context.Context, delivery *domain.WebhookDelivery) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries (id, webhook_id, event_id, event_type, attempted_at, delivered_at, status_code, error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		delivery.ID.String(),
		delivery.WebhookID.String(),
		delivery.EventID,
		string(delivery.EventType),
		delivery.AttemptedAt.Format(timeLayout),
		fmtTimePtr(delivery.DeliveredAt),
		delivery.StatusCode,
		delivery.Error,
	)
	return err
}

func (r *sqliteWebhookRepo) DeleteByID(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id.String())
	return err
}

func (r *sqliteWebhookRepo) MarkDeliveryResult(ctx context.Context, id uuid.UUID, deliveredAt *time.Time, lastError *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE webhooks
		 SET last_delivered_at = COALESCE(?, last_delivered_at), last_error = ?
		 WHERE id = ?`,
		fmtTimePtr(deliveredAt),
		lastError,
		id.String(),
	)
	return err
}

type webhookScanner interface {
	Scan(dest ...interface{}) error
}

func (r *sqliteWebhookRepo) scanWebhook(scanner webhookScanner) (*domain.WebhookSubscription, error) {
	var webhook domain.WebhookSubscription
	var idStr, url, secret, eventTypesRaw, createdAt, updatedAt string
	var enabled int
	var lastDeliveredAt, lastError *string

	err := scanner.Scan(
		&idStr,
		&url,
		&secret,
		&eventTypesRaw,
		&enabled,
		&lastDeliveredAt,
		&lastError,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	webhookID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("parse webhook id: %w", err)
	}
	created, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse webhook created_at: %w", err)
	}
	updated, err := parseTime(updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse webhook updated_at: %w", err)
	}
	eventTypes, err := unmarshalWebhookEventTypes(eventTypesRaw)
	if err != nil {
		return nil, err
	}
	decryptedSecret, err := r.decryptSecret(secret)
	if err != nil {
		return nil, err
	}

	webhook = domain.WebhookSubscription{
		ID:              webhookID,
		URL:             url,
		Secret:          decryptedSecret,
		EventTypes:      eventTypes,
		Enabled:         enabled == 1,
		LastDeliveredAt: timePtr(lastDeliveredAt),
		LastError:       lastError,
		CreatedAt:       created,
		UpdatedAt:       updated,
	}
	return &webhook, nil
}

func (r *sqliteWebhookRepo) encryptSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if r.secrets == nil {
		return "", security.ErrSecretEncryptionUnavailable
	}
	return r.secrets.Encrypt(value)
}

func (r *sqliteWebhookRepo) decryptSecret(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if r.secrets == nil {
		if security.IsEncryptedSecret(value) {
			return "", security.ErrSecretEncryptionUnavailable
		}
		return value, nil
	}
	return r.secrets.Decrypt(value)
}

func scanWebhookDelivery(scanner webhookScanner) (*domain.WebhookDelivery, error) {
	var delivery domain.WebhookDelivery
	var idStr, webhookIDStr, eventID, eventTypeRaw, attemptedAt string
	var deliveredAt, errorMessage *string
	var statusCode sql.NullInt64

	err := scanner.Scan(
		&idStr,
		&webhookIDStr,
		&eventID,
		&eventTypeRaw,
		&attemptedAt,
		&deliveredAt,
		&statusCode,
		&errorMessage,
	)
	if err != nil {
		return nil, err
	}

	deliveryID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("parse webhook delivery id: %w", err)
	}
	webhookID, err := uuid.Parse(webhookIDStr)
	if err != nil {
		return nil, fmt.Errorf("parse webhook delivery webhook_id: %w", err)
	}
	attempted, err := parseTime(attemptedAt)
	if err != nil {
		return nil, fmt.Errorf("parse webhook delivery attempted_at: %w", err)
	}

	delivery = domain.WebhookDelivery{
		ID:          deliveryID,
		WebhookID:   webhookID,
		EventID:     eventID,
		EventType:   domain.WebhookEventType(eventTypeRaw),
		AttemptedAt: attempted,
		DeliveredAt: timePtr(deliveredAt),
		Error:       errorMessage,
	}
	if statusCode.Valid {
		code := int(statusCode.Int64)
		delivery.StatusCode = &code
	}
	return &delivery, nil
}

func marshalWebhookEventTypes(eventTypes []domain.WebhookEventType) (string, error) {
	values := make([]string, 0, len(eventTypes))
	for _, eventType := range eventTypes {
		values = append(values, string(eventType))
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal webhook event types: %w", err)
	}
	return string(data), nil
}

func unmarshalWebhookEventTypes(raw string) ([]domain.WebhookEventType, error) {
	if raw == "" {
		return nil, nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, fmt.Errorf("unmarshal webhook event types: %w", err)
	}
	eventTypes := make([]domain.WebhookEventType, 0, len(values))
	for _, value := range values {
		eventTypes = append(eventTypes, domain.WebhookEventType(value))
	}
	return eventTypes, nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
