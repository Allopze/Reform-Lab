package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

func TestCreateIfUnderQuotaRejectsWhenUserWouldExceedQuota(t *testing.T) {
	t.Parallel()

	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewFileRepository(db).(*sqliteFileRepo)
	ctx := context.Background()
	userID := uuid.New()

	first := quotaTestFile(userID, nil, 90)
	if err := repo.CreateIfUnderQuota(ctx, first, 100); err != nil {
		t.Fatalf("first file should fit quota: %v", err)
	}

	second := quotaTestFile(userID, nil, 11)
	if err := repo.CreateIfUnderQuota(ctx, second, 100); err != domain.ErrQuotaExceeded {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}

	used, err := repo.CumulativeBytesByUser(ctx, userID)
	if err != nil {
		t.Fatalf("cumulative bytes: %v", err)
	}
	if used != 90 {
		t.Fatalf("expected only first file to persist, got %d bytes", used)
	}
}

func TestCreateIfUnderQuotaRejectsWhenGuestWouldExceedQuota(t *testing.T) {
	t.Parallel()

	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewFileRepository(db).(*sqliteFileRepo)
	ctx := context.Background()
	guestID := uuid.New()

	first := quotaTestFile(uuid.Nil, &guestID, 45)
	first.UserID = nil
	if err := repo.CreateIfUnderQuota(ctx, first, 50); err != nil {
		t.Fatalf("first guest file should fit quota: %v", err)
	}

	second := quotaTestFile(uuid.Nil, &guestID, 6)
	second.UserID = nil
	if err := repo.CreateIfUnderQuota(ctx, second, 50); err != domain.ErrQuotaExceeded {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func quotaTestFile(userID uuid.UUID, guestID *uuid.UUID, size int64) *domain.OriginalFile {
	id := uuid.New()
	var userIDPtr *uuid.UUID
	if userID != uuid.Nil {
		userIDPtr = &userID
	}
	return &domain.OriginalFile{
		ID:             id,
		UserID:         userIDPtr,
		GuestSessionID: guestID,
		InternalName:   id.String(),
		OriginalName:   "sample.txt",
		Size:           size,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  "text/plain",
			Family:    domain.FamilyDocument,
			Extension: "txt",
		},
		UploadedAt: time.Now().UTC(),
	}
}
