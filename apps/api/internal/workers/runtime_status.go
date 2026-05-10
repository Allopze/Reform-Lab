package workers

import (
	"context"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/repository"
)

func StartHeartbeatLoop(ctx context.Context, repo repository.WorkerStatusRepository, workerID, runtimeMode, queueMode string, engines map[string]bool, interval time.Duration) {
	if repo == nil || workerID == "" {
		return
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	beat := func() {
		now := time.Now().UTC()
		_ = repo.Heartbeat(ctx, repository.WorkerStatusSnapshot{
			ID:              workerID,
			RuntimeMode:     runtimeMode,
			QueueMode:       queueMode,
			LastHeartbeatAt: now,
			LastTaskStatus:  "idle",
			Engines:         engines,
		})
	}
	beat()
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				beat()
			}
		}
	}()
}
