package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// TaskHandler is the function signature that processes a task.
type TaskHandler func(ctx context.Context, taskType string, payload []byte) error

// InProcessQueue executes tasks in goroutines. Used for development without Redis.
type InProcessQueue struct {
	handler TaskHandler
	wg      sync.WaitGroup
}

// NewInProcessQueue creates a queue that dispatches tasks to handler in goroutines.
// If handler is nil, tasks are accepted but silently dropped (useful if the server
// includes an embedded worker).
func NewInProcessQueue(handler TaskHandler) *InProcessQueue {
	return &InProcessQueue{handler: handler}
}

func (q *InProcessQueue) Enqueue(ctx context.Context, taskType string, payload TaskPayload, opts TaskOptions) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	if q.handler == nil {
		return nil
	}

	q.wg.Add(1)
	go func() {
		defer q.wg.Done()
		taskCtx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
		_ = q.handler(taskCtx, taskType, data)
	}()

	return nil
}

func (q *InProcessQueue) Close() error {
	q.wg.Wait()
	return nil
}
