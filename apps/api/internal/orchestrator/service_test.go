package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

func newTestOrchestrator(t *testing.T) (*Service, repository.JobRepository) {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, testMigrationsPath(t)); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	jobs := repository.NewJobRepository(db)
	audit := repository.NewAuditRepository(db)
	q := queue.NewInProcessQueueWithLimit(nil, 1) // nil handler: accepts tasks silently

	return NewService(jobs, audit, q), jobs
}

func TestCreateAndEnqueue(t *testing.T) {
	svc, jobs := newTestOrchestrator(t)
	ctx := context.Background()

	cap := domain.Capability{
		ID:            "pdf-to-txt",
		TargetFormat:  "txt",
		SourceFormats: []string{"application/pdf"},
		Engine:        "poppler",
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
	}

	userID := uuid.New()
	fileID := uuid.New()
	job, err := svc.CreateAndEnqueue(ctx, &userID, fileID, cap, "/tmp/fake.pdf")
	if err != nil {
		t.Fatalf("CreateAndEnqueue: %v", err)
	}

	if job.Status != domain.JobQueued {
		t.Fatalf("expected queued, got %s", job.Status)
	}
	if job.FileID != fileID {
		t.Fatal("file ID mismatch")
	}
	if *job.UserID != userID {
		t.Fatal("user ID mismatch")
	}

	// Verify that the job was persisted.
	persisted, err := jobs.GetByID(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if persisted.Status != domain.JobQueued {
		t.Fatalf("expected persisted job to be queued, got %s", persisted.Status)
	}
}

func TestJobLifecycleTransitions(t *testing.T) {
	svc, jobs := newTestOrchestrator(t)
	ctx := context.Background()

	cap := domain.Capability{
		ID:            "pdf-to-txt",
		TargetFormat:  "txt",
		SourceFormats: []string{"application/pdf"},
		Engine:        "poppler",
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
	}

	tmpUID := uuid.New()
	job, err := svc.CreateAndEnqueue(ctx, &tmpUID, uuid.New(), cap, "/tmp/fake.pdf")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// queued → running
	if err := svc.MarkRunning(ctx, job.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	updated, _ := jobs.GetByID(ctx, job.ID)
	if updated.Status != domain.JobRunning {
		t.Fatalf("expected running, got %s", updated.Status)
	}

	// running → succeeded
	artifactID := uuid.New()
	if err := svc.MarkSucceeded(ctx, job.ID, artifactID); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}
	updated, _ = jobs.GetByID(ctx, job.ID)
	if updated.Status != domain.JobSucceeded {
		t.Fatalf("expected succeeded, got %s", updated.Status)
	}
	if updated.ArtifactID == nil || *updated.ArtifactID != artifactID {
		t.Fatal("artifact ID mismatch after success")
	}
}

func TestJobLifecycleFailure(t *testing.T) {
	svc, jobs := newTestOrchestrator(t)
	ctx := context.Background()

	cap := domain.Capability{
		ID:            "pdf-to-txt",
		TargetFormat:  "txt",
		SourceFormats: []string{"application/pdf"},
		Engine:        "poppler",
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
	}

	tmpUID := uuid.New()
	job, err := svc.CreateAndEnqueue(ctx, &tmpUID, uuid.New(), cap, "/tmp/fake.pdf")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.MarkRunning(ctx, job.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}

	if err := svc.MarkFailed(ctx, job.ID, "engine crashed"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	updated, _ := jobs.GetByID(ctx, job.ID)
	if updated.Status != domain.JobFailed {
		t.Fatalf("expected failed, got %s", updated.Status)
	}
	if updated.Error == nil || *updated.Error != "engine crashed" {
		t.Fatal("error message mismatch")
	}
}

func TestInvalidTransitionRejected(t *testing.T) {
	svc, _ := newTestOrchestrator(t)
	ctx := context.Background()

	cap := domain.Capability{
		ID:            "pdf-to-txt",
		TargetFormat:  "txt",
		SourceFormats: []string{"application/pdf"},
		Engine:        "poppler",
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
	}

	tmpUID := uuid.New()
	job, _ := svc.CreateAndEnqueue(ctx, &tmpUID, uuid.New(), cap, "/tmp/fake.pdf")

	// queued → succeeded — should be rejected
	artifactID := uuid.New()
	if err := svc.MarkSucceeded(ctx, job.ID, artifactID); err == nil {
		t.Fatal("expected error for invalid transition queued → succeeded")
	}
}

func TestCreateAndEnqueueRejectsWhenUserReachedActiveJobLimit(t *testing.T) {
	svc, _ := newTestOrchestrator(t)
	svc = NewService(svc.jobs, svc.audit, svc.q, WithMaxActiveJobsPerUser(1))
	ctx := context.Background()

	cap := domain.Capability{
		ID:            "pdf-to-txt",
		TargetFormat:  "txt",
		SourceFormats: []string{"application/pdf"},
		Engine:        "poppler",
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
	}

	userID := uuid.New()
	if _, err := svc.CreateAndEnqueue(ctx, &userID, uuid.New(), cap, "/tmp/first.pdf"); err != nil {
		t.Fatalf("first job should be accepted: %v", err)
	}
	if _, err := svc.CreateAndEnqueue(ctx, &userID, uuid.New(), cap, "/tmp/second.pdf"); err != domain.ErrTooManyActiveJobs {
		t.Fatalf("expected ErrTooManyActiveJobs, got %v", err)
	}
}
