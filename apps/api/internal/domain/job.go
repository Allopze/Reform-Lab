package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JobStatus is a closed set of states a job can be in.
type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobSucceeded JobStatus = "succeeded"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
	JobExpired   JobStatus = "expired"
)

// IsTerminal returns true if the status is a final state.
func (s JobStatus) IsTerminal() bool {
	switch s {
	case JobSucceeded, JobFailed, JobCancelled, JobExpired:
		return true
	}
	return false
}

// Job represents an async work unit for a conversion.
type Job struct {
	ID           uuid.UUID  `json:"id"`
	UserID       *uuid.UUID `json:"userId,omitempty"`
	FileID       uuid.UUID  `json:"fileId"`
	CapabilityID string     `json:"capabilityId"`
	OutputFormat string     `json:"outputFormat"`
	Status       JobStatus  `json:"status"`
	Progress     int        `json:"progress"`
	Error        *string    `json:"error,omitempty"`
	ArtifactID   *uuid.UUID `json:"artifactId,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// ValidTransition checks whether moving from current status to next is allowed.
func ValidTransition(from, to JobStatus) error {
	allowed := map[JobStatus][]JobStatus{
		JobQueued:  {JobRunning, JobCancelled, JobExpired},
		JobRunning: {JobSucceeded, JobFailed, JobCancelled},
	}

	targets, ok := allowed[from]
	if !ok {
		return fmt.Errorf("no transitions allowed from terminal state %q", from)
	}
	for _, t := range targets {
		if t == to {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %q to %q", from, to)
}
