package queue

import (
	"context"
	"time"
)

// TaskPayload is the data sent to workers via the job queue.
type TaskPayload struct {
	JobID        string `json:"jobId"`
	UserID       string `json:"userId,omitempty"`
	FileID       string `json:"fileId"`
	CapabilityID string `json:"capabilityId"`
	InputPath    string `json:"inputPath"`
	OutputFormat string `json:"outputFormat"`
}

// EmailTaskPayload is the data sent for email delivery tasks.
type EmailTaskPayload struct {
	TemplateKey string            `json:"templateKey"`
	To          string            `json:"to"`
	Vars        map[string]string `json:"vars"`
}

// EmailTaskType is the task type used for email delivery via the queue.
const EmailTaskType = "email:send"

// TaskOptions configures retry and timeout for a task.
type TaskOptions struct {
	MaxRetries int
	Timeout    time.Duration
}

// JobQueue abstracts the enqueueing of conversion tasks.
type JobQueue interface {
	Enqueue(ctx context.Context, taskType string, payload TaskPayload, opts TaskOptions) error
	EnqueueEmail(ctx context.Context, payload EmailTaskPayload, opts TaskOptions) error
	Close() error
}
