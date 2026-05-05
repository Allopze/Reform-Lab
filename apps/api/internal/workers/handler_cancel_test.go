package workers

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/database"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/google/uuid"
)

var (
	handlerTestMetrics     *observability.Metrics
	handlerTestMetricsOnce sync.Once
)

type blockingEngine struct {
	started   chan struct{}
	startOnce sync.Once
	cancelled bool
	mu        sync.Mutex
}

func (e *blockingEngine) Execute(ctx context.Context, _, _, _ string) (string, error) {
	e.startOnce.Do(func() { close(e.started) })
	<-ctx.Done()
	e.mu.Lock()
	e.cancelled = true
	e.mu.Unlock()
	return "", ctx.Err()
}

func (e *blockingEngine) wasCancelled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cancelled
}

func TestProcessPayloadCancelsRunningJob(t *testing.T) {
	tmpDir := t.TempDir()
	db, err := database.Open(filepath.Join(tmpDir, "worker-test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("migrate db: %v", err)
	}

	store, err := storage.NewFilesystem(filepath.Join(tmpDir, "storage"))
	if err != nil {
		t.Fatalf("init storage: %v", err)
	}

	jobRepo := repository.NewJobRepository(db)
	auditRepo := repository.NewAuditRepository(db)
	artifactRepo := repository.NewArtifactRepository(db)
	jobQueue := queue.NewInProcessQueueWithLimit(nil, 1)
	defer jobQueue.Close()

	orch := orchestrator.NewService(jobRepo, auditRepo, jobQueue)
	capability := capabilities.ByID("image-to-png")
	if capability == nil {
		t.Fatal("expected image-to-png capability")
	}

	job, err := orch.CreateAndEnqueue(context.Background(), nil, uuid.New(), *capability, filepath.Join(tmpDir, "input.png"), 1024)
	if err != nil {
		t.Fatalf("create job: %v", err)
	}

	engine := &blockingEngine{started: make(chan struct{})}
	registry := NewRegistry()
	registry.Register("image-to-png", engine)

	handlerTestMetricsOnce.Do(func() {
		handlerTestMetrics = observability.NewMetrics()
	})

	handler := &Handler{
		Registry:    registry,
		Store:       store,
		Artifacts:   artifactRepo,
		Audit:       auditRepo,
		Orch:        orch,
		Logger:      observability.NewLogger("disabled"),
		Metrics:     handlerTestMetrics,
		ArtifactTTL: time.Hour,
	}

	payload, err := json.Marshal(queue.TaskPayload{
		JobID:        job.ID.String(),
		FileID:       job.FileID.String(),
		CapabilityID: "image-to-png",
		InputPath:    filepath.Join(tmpDir, "input.png"),
		OutputFormat: "png",
		InputSize:    1024,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- handler.ProcessPayload(context.Background(), "conversion:image-to-png", payload)
	}()

	select {
	case <-engine.started:
	case <-time.After(2 * time.Second):
		t.Fatal("engine did not start")
	}

	if err := orch.CancelJob(context.Background(), job.ID); err != nil {
		t.Fatalf("cancel job: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("expected nil error after cancellation, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not stop after cancellation")
	}

	finalJob, err := orch.GetJob(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("get final job: %v", err)
	}
	if finalJob.Status != domain.JobCancelled {
		t.Fatalf("expected final status cancelled, got %s", finalJob.Status)
	}
	if !engine.wasCancelled() {
		t.Fatal("expected engine context to be cancelled")
	}
}
