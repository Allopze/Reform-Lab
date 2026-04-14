package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/security"
	"github.com/google/uuid"
)

func TestWebhookRepositoryEncryptsSecretAtRest(t *testing.T) {
	t.Parallel()

	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	keeper, err := security.NewSecretKeeper("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewSecretKeeper: %v", err)
	}
	repo := NewWebhookRepository(db, WithSecretKeeper(keeper))

	webhook := &domain.WebhookSubscription{
		ID:         uuid.New(),
		URL:        "https://hooks.example.com/reform",
		Secret:     "super-secret",
		EventTypes: []domain.WebhookEventType{domain.WebhookEventJobCompleted},
		Enabled:    true,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := repo.Create(context.Background(), webhook); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var storedSecret string
	if err := db.QueryRowContext(context.Background(), `SELECT secret FROM webhooks WHERE id = ?`, webhook.ID.String()).Scan(&storedSecret); err != nil {
		t.Fatalf("query raw secret: %v", err)
	}
	if storedSecret == "super-secret" {
		t.Fatal("expected stored webhook secret to be encrypted")
	}

	loaded, err := repo.GetByID(context.Background(), webhook.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if loaded.Secret != "super-secret" {
		t.Fatalf("expected decrypted webhook secret, got %q", loaded.Secret)
	}
}

func TestWebhookRepositoryRejectsSecretPersistenceWithoutKeeper(t *testing.T) {
	t.Parallel()

	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewWebhookRepository(db)
	webhook := &domain.WebhookSubscription{
		ID:         uuid.New(),
		URL:        "https://hooks.example.com/reform",
		Secret:     "super-secret",
		EventTypes: []domain.WebhookEventType{domain.WebhookEventJobCompleted},
		Enabled:    true,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := repo.Create(context.Background(), webhook); err != security.ErrSecretEncryptionUnavailable {
		t.Fatalf("expected ErrSecretEncryptionUnavailable, got %v", err)
	}
}
