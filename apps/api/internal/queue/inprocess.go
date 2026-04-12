package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
)

// TaskHandler is the function signature that processes a task.
type TaskHandler func(ctx context.Context, taskType string, payload []byte) error

// InProcessQueue executes tasks in goroutines. Used for development without Redis.
type InProcessQueue struct {
	handler TaskHandler
	wg      sync.WaitGroup
	sem     chan struct{}
}

// NewInProcessQueue creates a queue that dispatches tasks to handler in goroutines.
// If handler is nil, tasks are accepted but silently dropped (useful if the server
// includes an embedded worker).
func NewInProcessQueue(handler TaskHandler) *InProcessQueue {
	return NewInProcessQueueWithLimit(handler, 2)
}

// NewInProcessQueueWithLimit creates an in-process queue with bounded concurrency.
func NewInProcessQueueWithLimit(handler TaskHandler, maxConcurrent int) *InProcessQueue {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &InProcessQueue{handler: handler, sem: make(chan struct{}, maxConcurrent)}
}

func (q *InProcessQueue) Enqueue(ctx context.Context, taskType string, payload TaskPayload, opts TaskOptions) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	return q.enqueueRaw(taskType, data, opts)
}

func (q *InProcessQueue) EnqueueEmail(ctx context.Context, payload EmailTaskPayload, opts TaskOptions) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email payload: %w", err)
	}

	return q.enqueueRaw(EmailTaskType, data, opts)
}

func (q *InProcessQueue) enqueueRaw(taskType string, data []byte, opts TaskOptions) error {
	if q.handler == nil {
		return nil
	}

	q.wg.Add(1)
	q.sem <- struct{}{}
	go func() {
		defer q.wg.Done()
		defer func() { <-q.sem }()
		taskCtx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
		defer cancel()
		if err := q.handler(taskCtx, taskType, data); err != nil {
			log.Printf("[InProcessQueue] task %s failed: %v", taskType, err)
		}
	}()

	return nil
}

func (q *InProcessQueue) Close() error {
	q.wg.Wait()
	return nil
}
