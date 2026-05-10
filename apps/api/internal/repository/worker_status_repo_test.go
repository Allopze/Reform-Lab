package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/database"
)

func TestWorkerStatusHeartbeatPersistsEngineAvailability(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewWorkerStatusRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := repo.Heartbeat(ctx, WorkerStatusSnapshot{
		ID:              "worker-1",
		RuntimeMode:     "standalone",
		QueueMode:       "redis",
		LastHeartbeatAt: now,
		LastTaskStatus:  "idle",
		Engines: map[string]bool{
			"ffmpeg":      true,
			"libreoffice": false,
		},
	}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	workers, err := repo.List(ctx, 1)
	if err != nil {
		t.Fatalf("list workers: %v", err)
	}
	if len(workers) != 1 {
		t.Fatalf("expected 1 worker, got %d", len(workers))
	}
	if !workers[0].Engines["ffmpeg"] {
		t.Fatalf("expected ffmpeg engine to be available, got %#v", workers[0].Engines)
	}
	if workers[0].Engines["libreoffice"] {
		t.Fatalf("expected libreoffice engine to be unavailable, got %#v", workers[0].Engines)
	}
}
