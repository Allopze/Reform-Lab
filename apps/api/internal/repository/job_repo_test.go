package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

func TestCreateIfUnderGuestLimitRejectsWhenSessionReachedLimit(t *testing.T) {
	t.Parallel()

	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	migrationsPath := filepath.Join("..", "..", "migrations")
	if err := database.Migrate(db, migrationsPath); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewJobRepository(db)
	ctx := context.Background()
	guestSessionID := uuid.New()

	firstFileID := insertGuestFile(t, db, guestSessionID)
	firstJob := &domain.Job{
		ID:           uuid.New(),
		FileID:       firstFileID,
		CapabilityID: "image-png-to-jpg",
		OutputFormat: "jpg",
		Status:       domain.JobQueued,
		Progress:     0,
		CreatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateIfUnderGuestLimit(ctx, guestSessionID, firstJob, 1); err != nil {
		t.Fatalf("first guest job should be accepted: %v", err)
	}

	secondFileID := insertGuestFile(t, db, guestSessionID)
	secondJob := &domain.Job{
		ID:           uuid.New(),
		FileID:       secondFileID,
		CapabilityID: "image-png-to-jpg",
		OutputFormat: "jpg",
		Status:       domain.JobQueued,
		Progress:     0,
		CreatedAt:    time.Now().UTC(),
	}
	if err := repo.CreateIfUnderGuestLimit(ctx, guestSessionID, secondJob, 1); err != domain.ErrTooManyActiveJobs {
		t.Fatalf("expected ErrTooManyActiveJobs, got %v", err)
	}
	count, err := repo.CountActiveByGuestSession(ctx, guestSessionID)
	if err != nil {
		t.Fatalf("CountActiveByGuestSession: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 persisted active guest job, got %d", count)
	}
}

func insertGuestFile(t *testing.T, db *sql.DB, guestSessionID uuid.UUID) uuid.UUID {
	t.Helper()
	fileID := uuid.New()
	ctx := context.Background()
	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO files (id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at, guest_session_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		fileID.String(), fileID.String()+".png", "sample.png", 128, "image/png", string(domain.FamilyImage), "png", `{}`,
		time.Now().UTC().Format(timeLayout), guestSessionID.String(),
	); err != nil {
		t.Fatalf("insert guest file: %v", err)
	}
	return fileID
}
