package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

// AsynqQueue implements JobQueue using Redis + Asynq.
type AsynqQueue struct {
	client *asynq.Client
}

// NewAsynqQueue creates a queue backed by Redis.
func NewAsynqQueue(redisURL string) (*AsynqQueue, error) {
	opt, err := asynq.ParseRedisURI(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}
	return &AsynqQueue{client: asynq.NewClient(opt)}, nil
}

func (q *AsynqQueue) Enqueue(_ context.Context, taskType string, payload TaskPayload, opts TaskOptions) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(taskType, data,
		asynq.MaxRetry(opts.MaxRetries),
		asynq.Timeout(opts.Timeout),
	)
	_, err = q.client.Enqueue(task)
	return err
}

func (q *AsynqQueue) EnqueueEmail(_ context.Context, payload EmailTaskPayload, opts TaskOptions) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal email payload: %w", err)
	}
	task := asynq.NewTask(EmailTaskType, data,
		asynq.MaxRetry(opts.MaxRetries),
		asynq.Timeout(opts.Timeout),
	)
	_, err = q.client.Enqueue(task)
	return err
}

func (q *AsynqQueue) Close() error {
	return q.client.Close()
}
