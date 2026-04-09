package orchestrator

import (
	"context"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

// Service manages job lifecycle: creation, status updates, and queuing.
type Service struct {
	jobs                 repository.JobRepository
	audit                repository.AuditRepository
	q                    queue.JobQueue
	maxActiveJobsPerUser int
}

type Option func(*Service)

func WithMaxActiveJobsPerUser(limit int) Option {
	return func(s *Service) {
		s.maxActiveJobsPerUser = limit
	}
}

// NewService creates an orchestrator service.
func NewService(jobs repository.JobRepository, audit repository.AuditRepository, q queue.JobQueue, opts ...Option) *Service {
	svc := &Service{jobs: jobs, audit: audit, q: q}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

// CreateAndEnqueue persists a new job and enqueues it for processing.
// userID may be nil for system-owned work outside the public API.
func (s *Service) CreateAndEnqueue(ctx context.Context, userID *uuid.UUID, fileID uuid.UUID, cap domain.Capability, inputPath string) (*domain.Job, error) {
	if err := s.enforceActiveJobLimit(ctx, userID); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	job := domain.Job{
		ID:           uuid.New(),
		UserID:       userID,
		FileID:       fileID,
		CapabilityID: cap.ID,
		OutputFormat: cap.TargetFormat,
		Status:       domain.JobQueued,
		Progress:     0,
		CreatedAt:    now,
	}

	if err := s.jobs.Create(ctx, &job); err != nil {
		return nil, fmt.Errorf("persist job: %w", err)
	}

	var userIDStr string
	if userID != nil {
		userIDStr = userID.String()
	}
	taskType := fmt.Sprintf("conversion:%s", cap.ID)
	payload := queue.TaskPayload{
		JobID:        job.ID.String(),
		UserID:       userIDStr,
		FileID:       fileID.String(),
		CapabilityID: cap.ID,
		InputPath:    inputPath,
		OutputFormat: cap.TargetFormat,
	}
	opts := queue.TaskOptions{
		MaxRetries: cap.ExecutionLimits.MaxRetries,
		Timeout:    time.Duration(cap.ExecutionLimits.TimeoutSeconds) * time.Second,
	}

	if err := s.q.Enqueue(ctx, taskType, payload, opts); err != nil {
		// Mark the job as failed so it's not orphaned in queued state.
		errMsg := fmt.Sprintf("enqueue failed: %v", err)
		job.Status = domain.JobFailed
		job.Error = &errMsg
		now := time.Now().UTC()
		job.CompletedAt = &now
		_ = s.jobs.Update(ctx, &job)
		return nil, fmt.Errorf("enqueue task: %w", err)
	}

	_ = s.audit.Create(ctx, &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: domain.AuditJobCreated,
		FileID:    &fileID,
		JobID:     &job.ID,
		Details:   map[string]interface{}{"capabilityId": cap.ID},
		CreatedAt: now,
	})

	return &job, nil
}

func (s *Service) enforceActiveJobLimit(ctx context.Context, userID *uuid.UUID) error {
	if userID == nil || s.maxActiveJobsPerUser <= 0 {
		return nil
	}
	count, err := s.jobs.CountActiveByUser(ctx, *userID)
	if err != nil {
		return fmt.Errorf("count active jobs: %w", err)
	}
	if count >= s.maxActiveJobsPerUser {
		return domain.ErrTooManyActiveJobs
	}
	return nil
}

// GetJob returns a job by ID.
func (s *Service) GetJob(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	return s.jobs.GetByID(ctx, id)
}

// UpdateProgress updates the progress percentage for a running job.
// progress must be between 0 and 100. Only running jobs can have progress updated.
func (s *Service) UpdateProgress(ctx context.Context, id uuid.UUID, progress int) error {
	if progress < 0 || progress > 100 {
		return fmt.Errorf("progress must be between 0 and 100")
	}
	job, err := s.jobs.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if job.Status != domain.JobRunning {
		return nil // silently ignore progress for non-running jobs
	}
	job.Progress = progress
	return s.jobs.Update(ctx, job)
}

// MarkRunning transitions a job to running.
func (s *Service) MarkRunning(ctx context.Context, id uuid.UUID) error {
	return s.transitionJob(ctx, id, domain.JobRunning, nil, nil)
}

// MarkSucceeded transitions a job to succeeded with an artifact reference.
func (s *Service) MarkSucceeded(ctx context.Context, id uuid.UUID, artifactID uuid.UUID) error {
	return s.transitionJob(ctx, id, domain.JobSucceeded, &artifactID, nil)
}

// MarkFailed transitions a job to failed with an error message.
func (s *Service) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	return s.transitionJob(ctx, id, domain.JobFailed, nil, &errMsg)
}

// CancelJob transitions a queued or running job to cancelled.
// Only the owning user or an admin may cancel.
func (s *Service) CancelJob(ctx context.Context, id uuid.UUID) error {
	return s.transitionJob(ctx, id, domain.JobCancelled, nil, nil)
}

// RetryFailedJob creates a new queued job from a previously failed job.
func (s *Service) RetryFailedJob(ctx context.Context, sourceJob *domain.Job, cap domain.Capability, inputPath string) (*domain.Job, error) {
	if sourceJob == nil {
		return nil, fmt.Errorf("source job is required")
	}
	if sourceJob.Status != domain.JobFailed {
		return nil, fmt.Errorf("%w: retry only allowed for failed jobs", domain.ErrInvalidTransition)
	}

	retryJob, err := s.CreateAndEnqueue(ctx, sourceJob.UserID, sourceJob.FileID, cap, inputPath)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Create(ctx, &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: domain.AuditJobRetried,
		FileID:    &sourceJob.FileID,
		JobID:     &retryJob.ID,
		Details: map[string]interface{}{
			"sourceJobId":  sourceJob.ID.String(),
			"capabilityId": sourceJob.CapabilityID,
		},
		CreatedAt: time.Now().UTC(),
	})

	return retryJob, nil
}

func (s *Service) transitionJob(ctx context.Context, id uuid.UUID, to domain.JobStatus, artifactID *uuid.UUID, errMsg *string) error {
	job, err := s.jobs.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := domain.ValidTransition(job.Status, to); err != nil {
		return fmt.Errorf("%w: %v", domain.ErrInvalidTransition, err)
	}

	now := time.Now().UTC()
	job.Status = to

	switch to {
	case domain.JobRunning:
		job.StartedAt = &now
		job.Progress = 10 // starting
	case domain.JobSucceeded:
		job.CompletedAt = &now
		job.Progress = 100
		job.ArtifactID = artifactID
	case domain.JobFailed:
		job.CompletedAt = &now
		job.Error = errMsg
	case domain.JobCancelled:
		job.CompletedAt = &now
	}

	if err := s.jobs.Update(ctx, job); err != nil {
		return err
	}

	var eventType domain.AuditEventType
	details := map[string]interface{}{}
	switch to {
	case domain.JobRunning:
		eventType = domain.AuditJobStarted
	case domain.JobSucceeded:
		eventType = domain.AuditJobCompleted
		if artifactID != nil {
			details["artifactId"] = artifactID.String()
		}
	case domain.JobFailed:
		eventType = domain.AuditJobFailed
		if errMsg != nil {
			details["error"] = *errMsg
		}
	case domain.JobCancelled:
		eventType = domain.AuditJobCancelled
	}
	if eventType != "" {
		_ = s.audit.Create(ctx, &domain.AuditEvent{
			ID:        uuid.New(),
			EventType: eventType,
			FileID:    &job.FileID,
			JobID:     &job.ID,
			Details:   details,
			CreatedAt: now,
		})
	}

	return nil
}
