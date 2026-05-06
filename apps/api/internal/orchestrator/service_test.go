package orchestrator

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

type spyQueue struct {
	mu   sync.Mutex
	opts []queue.TaskOptions
}

func (q *spyQueue) Enqueue(_ context.Context, _ string, _ queue.TaskPayload, opts queue.TaskOptions) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.opts = append(q.opts, opts)
	return nil
}

func (q *spyQueue) EnqueueEmail(context.Context, queue.EmailTaskPayload, queue.TaskOptions) error {
	return nil
}

func (q *spyQueue) EnqueueWebhook(context.Context, queue.WebhookTaskPayload, queue.TaskOptions) error {
	return nil
}

func (q *spyQueue) Close() error {
	return nil
}

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

func newTestOrchestratorWithQueue(t *testing.T, q queue.JobQueue) (*Service, repository.JobRepository) {
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
	job, err := svc.CreateAndEnqueue(ctx, &userID, fileID, cap, "/tmp/fake.pdf", 1024)
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

func TestCreateAndEnqueueDisablesQueueAutoRetries(t *testing.T) {
	q := &spyQueue{}
	svc, _ := newTestOrchestratorWithQueue(t, q)
	ctx := context.Background()

	cap := domain.Capability{
		ID:            "pdf-to-txt",
		TargetFormat:  "txt",
		SourceFormats: []string{"application/pdf"},
		Engine:        "poppler",
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     3,
		},
	}

	userID := uuid.New()
	if _, err := svc.CreateAndEnqueue(ctx, &userID, uuid.New(), cap, "/tmp/fake.pdf", 1024); err != nil {
		t.Fatalf("CreateAndEnqueue: %v", err)
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.opts) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(q.opts))
	}
	if q.opts[0].MaxRetries != 0 {
		t.Fatalf("expected queue auto retries disabled, got %d", q.opts[0].MaxRetries)
	}
	if q.opts[0].Timeout != time.Minute {
		t.Fatalf("expected timeout from capability, got %s", q.opts[0].Timeout)
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
	job, err := svc.CreateAndEnqueue(ctx, &tmpUID, uuid.New(), cap, "/tmp/fake.pdf", 1024)
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
	job, err := svc.CreateAndEnqueue(ctx, &tmpUID, uuid.New(), cap, "/tmp/fake.pdf", 1024)
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
	job, _ := svc.CreateAndEnqueue(ctx, &tmpUID, uuid.New(), cap, "/tmp/fake.pdf", 1024)

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
	if _, err := svc.CreateAndEnqueue(ctx, &userID, uuid.New(), cap, "/tmp/first.pdf", 1024); err != nil {
		t.Fatalf("first job should be accepted: %v", err)
	}
	if _, err := svc.CreateAndEnqueue(ctx, &userID, uuid.New(), cap, "/tmp/second.pdf", 1024); err != domain.ErrTooManyActiveJobs {
		t.Fatalf("expected ErrTooManyActiveJobs, got %v", err)
	}
}

// mockNotifier records calls to NotifyJobCompleted and NotifyJobFailed.
type mockNotifier struct {
	completedJobs []*domain.Job
	failedJobs    []*domain.Job
}

func (m *mockNotifier) NotifyJobCompleted(_ context.Context, job *domain.Job) error {
	m.completedJobs = append(m.completedJobs, job)
	return nil
}
func (m *mockNotifier) NotifyJobFailed(_ context.Context, job *domain.Job) error {
	m.failedJobs = append(m.failedJobs, job)
	return nil
}

func TestNotifierCalledOnSuccess(t *testing.T) {
	svc, _ := newTestOrchestrator(t)
	notifier := &mockNotifier{}
	svc = NewService(svc.jobs, svc.audit, svc.q, WithNotifier(notifier))
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

	uid := uuid.New()
	job, err := svc.CreateAndEnqueue(ctx, &uid, uuid.New(), cap, "/tmp/fake.pdf", 1024)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.MarkRunning(ctx, job.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := svc.MarkSucceeded(ctx, job.ID, uuid.New()); err != nil {
		t.Fatalf("mark succeeded: %v", err)
	}

	if len(notifier.completedJobs) != 1 {
		t.Fatalf("expected 1 completed notification, got %d", len(notifier.completedJobs))
	}
	if notifier.completedJobs[0].ID != job.ID {
		t.Fatal("notified wrong job ID")
	}
	if len(notifier.failedJobs) != 0 {
		t.Fatalf("expected 0 failed notifications, got %d", len(notifier.failedJobs))
	}
}

func TestNotifierCalledOnFailure(t *testing.T) {
	svc, _ := newTestOrchestrator(t)
	notifier := &mockNotifier{}
	svc = NewService(svc.jobs, svc.audit, svc.q, WithNotifier(notifier))
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

	uid := uuid.New()
	job, err := svc.CreateAndEnqueue(ctx, &uid, uuid.New(), cap, "/tmp/fake.pdf", 1024)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.MarkRunning(ctx, job.ID); err != nil {
		t.Fatalf("mark running: %v", err)
	}
	if err := svc.MarkFailed(ctx, job.ID, "boom"); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	if len(notifier.failedJobs) != 1 {
		t.Fatalf("expected 1 failed notification, got %d", len(notifier.failedJobs))
	}
	if notifier.failedJobs[0].ID != job.ID {
		t.Fatal("notified wrong job ID")
	}
	if len(notifier.completedJobs) != 0 {
		t.Fatalf("expected 0 completed notifications, got %d", len(notifier.completedJobs))
	}
}
