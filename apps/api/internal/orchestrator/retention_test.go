package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

func testMigrationsPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func TestRetentionServicePurgesExpiredArtifacts(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, testMigrationsPath(t)); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	jobRepo := repository.NewJobRepository(db)
	artifactRepo := repository.NewArtifactRepository(db)
	logger := observability.NewLogger("debug")
	service := NewRetentionService(artifactRepo, jobRepo, logger)

	jobID := uuid.New()
	fileID := uuid.New()
	artifactID := uuid.New()
	ownerID := uuid.New()
	now := time.Now().UTC()
	artifactDir := filepath.Join(t.TempDir(), artifactID.String())
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("create artifact dir: %v", err)
	}
	artifactPath := filepath.Join(artifactDir, "converted.txt")
	if err := os.WriteFile(artifactPath, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write artifact file: %v", err)
	}

	job := &domain.Job{
		ID:           jobID,
		UserID:       &ownerID,
		FileID:       fileID,
		CapabilityID: "pdf-to-txt",
		OutputFormat: "txt",
		Status:       domain.JobSucceeded,
		Progress:     100,
		ArtifactID:   &artifactID,
		CreatedAt:    now.Add(-2 * time.Hour),
		CompletedAt:  ptrTime(now.Add(-90 * time.Minute)),
	}
	if err := jobRepo.Create(ctx, job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if err := jobRepo.Update(ctx, job); err != nil {
		t.Fatalf("persist succeeded job state: %v", err)
	}

	artifact := &domain.Artifact{
		ID:          artifactID,
		UserID:      &ownerID,
		JobID:       jobID,
		FileID:      fileID,
		FileName:    "converted.txt",
		MIMEType:    "text/plain",
		Size:        11,
		StoragePath: artifactPath,
		CreatedAt:   now.Add(-90 * time.Minute),
		ExpiresAt:   now.Add(-10 * time.Minute),
	}
	if err := artifactRepo.Create(ctx, artifact); err != nil {
		t.Fatalf("create artifact: %v", err)
	}

	service.runOnce(ctx)

	if _, err := os.Stat(artifactDir); !os.IsNotExist(err) {
		t.Fatalf("expected artifact directory to be removed, stat err=%v", err)
	}
	if _, err := artifactRepo.GetByID(ctx, artifactID); err == nil {
		t.Fatal("expected artifact record to be deleted")
	}

	updatedJob, err := jobRepo.GetByID(ctx, jobID)
	if err != nil {
		t.Fatalf("reload job: %v", err)
	}
	if updatedJob.Status != domain.JobExpired {
		t.Fatalf("expected job to be expired, got %q", updatedJob.Status)
	}
	if updatedJob.ArtifactID != nil {
		t.Fatal("expected expired job artifact reference to be cleared")
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
