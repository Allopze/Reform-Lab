package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

// JobNotifier receives notifications when a job reaches a terminal state.
// Implementations must be safe for best-effort use (errors are logged, not fatal).
type JobNotifier interface {
	NotifyJobCompleted(ctx context.Context, job *domain.Job) error
	NotifyJobFailed(ctx context.Context, job *domain.Job) error
}

type BatchRequest struct {
	FileID     uuid.UUID
	Capability domain.Capability
	InputPath  string
	InputSize  int64
}

// Service manages job lifecycle: creation, status updates, and queuing.
type Service struct {
	jobs                  repository.JobRepository
	audit                 repository.AuditRepository
	q                     queue.JobQueue
	runtimeControls       repository.RuntimeControlRepository
	maxActiveJobsPerUser  int
	maxActiveJobsPerGuest int
	notifier              JobNotifier
}

type Option func(*Service)

func WithMaxActiveJobsPerUser(limit int) Option {
	return func(s *Service) {
		s.maxActiveJobsPerUser = limit
	}
}

func WithMaxActiveJobsPerGuestSession(limit int) Option {
	return func(s *Service) {
		s.maxActiveJobsPerGuest = limit
	}
}

// WithNotifier sets an optional notifier for job lifecycle events.
func WithNotifier(n JobNotifier) Option {
	return func(s *Service) {
		s.notifier = n
	}
}

func WithRuntimeControls(repo repository.RuntimeControlRepository) Option {
	return func(s *Service) {
		s.runtimeControls = repo
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

func NewMultiNotifier(notifiers ...JobNotifier) JobNotifier {
	filtered := make([]JobNotifier, 0, len(notifiers))
	for _, notifier := range notifiers {
		if notifier != nil {
			filtered = append(filtered, notifier)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return multiNotifier{notifiers: filtered}
}

type multiNotifier struct {
	notifiers []JobNotifier
}

func (m multiNotifier) NotifyJobCompleted(ctx context.Context, job *domain.Job) error {
	for _, notifier := range m.notifiers {
		_ = notifier.NotifyJobCompleted(ctx, job)
	}
	return nil
}

func (m multiNotifier) NotifyJobFailed(ctx context.Context, job *domain.Job) error {
	for _, notifier := range m.notifiers {
		_ = notifier.NotifyJobFailed(ctx, job)
	}
	return nil
}

// CreateAndEnqueue persists a new job and enqueues it for processing.
// userID may be nil for system-owned work outside the public API.
func (s *Service) CreateAndEnqueue(ctx context.Context, userID *uuid.UUID, fileID uuid.UUID, cap domain.Capability, inputPath string, inputSize int64) (*domain.Job, error) {
	return s.createAndEnqueue(ctx, userID, nil, fileID, cap, inputPath, inputSize)
}

func (s *Service) CreateAndEnqueueForGuest(ctx context.Context, guestSessionID uuid.UUID, fileID uuid.UUID, cap domain.Capability, inputPath string, inputSize int64) (*domain.Job, error) {
	return s.createAndEnqueue(ctx, nil, &guestSessionID, fileID, cap, inputPath, inputSize)
}

func (s *Service) createAndEnqueue(ctx context.Context, userID *uuid.UUID, guestSessionID *uuid.UUID, fileID uuid.UUID, cap domain.Capability, inputPath string, inputSize int64) (*domain.Job, error) {
	return s.createAndEnqueueWithAttempt(ctx, userID, guestSessionID, fileID, cap, inputPath, inputSize, nil, 0)
}

func (s *Service) createAndEnqueueWithAttempt(ctx context.Context, userID *uuid.UUID, guestSessionID *uuid.UUID, fileID uuid.UUID, cap domain.Capability, inputPath string, inputSize int64, sourceJobID *uuid.UUID, attemptNumber int) (*domain.Job, error) {
	if s.runtimeControls != nil {
		state, err := s.runtimeControls.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("load runtime controls: %w", err)
		}
		if state.JobIntakePaused {
			return nil, domain.ErrJobIntakePaused
		}
	}

	now := time.Now().UTC()
	job := domain.Job{
		ID:            uuid.New(),
		UserID:        userID,
		FileID:        fileID,
		SourceJobID:   sourceJobID,
		CapabilityID:  cap.ID,
		OutputFormat:  cap.TargetFormat,
		AttemptNumber: attemptNumber,
		Status:        domain.JobQueued,
		Progress:      0,
		CreatedAt:     now,
	}

	// Atomically check the active job limit and create the job in a single transaction.
	var err error
	if guestSessionID != nil {
		err = s.jobs.CreateIfUnderGuestLimit(ctx, *guestSessionID, &job, s.maxActiveJobsPerGuest)
	} else {
		err = s.jobs.CreateIfUnderLimit(ctx, &job, s.maxActiveJobsPerUser)
	}
	if err != nil {
		if errors.Is(err, domain.ErrTooManyActiveJobs) {
			return nil, err
		}
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
		InputSize:    inputSize,
	}
	opts := queue.TaskOptions{
		MaxRetries: 0,
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

func (s *Service) CreateAndEnqueueBatch(ctx context.Context, userID *uuid.UUID, requests []BatchRequest) ([]domain.Job, error) {
	return s.createAndEnqueueBatch(ctx, userID, nil, requests)
}

func (s *Service) CreateAndEnqueueBatchForGuest(ctx context.Context, guestSessionID uuid.UUID, requests []BatchRequest) ([]domain.Job, error) {
	return s.createAndEnqueueBatch(ctx, nil, &guestSessionID, requests)
}

func (s *Service) createAndEnqueueBatch(ctx context.Context, userID *uuid.UUID, guestSessionID *uuid.UUID, requests []BatchRequest) ([]domain.Job, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	if s.runtimeControls != nil {
		state, err := s.runtimeControls.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("load runtime controls: %w", err)
		}
		if state.JobIntakePaused {
			return nil, domain.ErrJobIntakePaused
		}
	}

	now := time.Now().UTC()
	jobs := make([]domain.Job, len(requests))
	jobRefs := make([]*domain.Job, 0, len(requests))
	for index, request := range requests {
		jobs[index] = domain.Job{
			ID:           uuid.New(),
			UserID:       userID,
			FileID:       request.FileID,
			CapabilityID: request.Capability.ID,
			OutputFormat: request.Capability.TargetFormat,
			Status:       domain.JobQueued,
			Progress:     0,
			CreatedAt:    now,
		}
		jobRefs = append(jobRefs, &jobs[index])
	}

	var err error
	if guestSessionID != nil {
		err = s.jobs.CreateManyIfUnderGuestLimit(ctx, *guestSessionID, jobRefs, s.maxActiveJobsPerGuest)
	} else {
		err = s.jobs.CreateManyIfUnderLimit(ctx, jobRefs, s.maxActiveJobsPerUser)
	}
	if err != nil {
		if errors.Is(err, domain.ErrTooManyActiveJobs) {
			return nil, err
		}
		return nil, fmt.Errorf("persist jobs: %w", err)
	}

	var userIDStr string
	if userID != nil {
		userIDStr = userID.String()
	}

	for index := range jobs {
		request := requests[index]
		job := &jobs[index]
		taskType := fmt.Sprintf("conversion:%s", request.Capability.ID)
		payload := queue.TaskPayload{
			JobID:        job.ID.String(),
			UserID:       userIDStr,
			FileID:       request.FileID.String(),
			CapabilityID: request.Capability.ID,
			InputPath:    request.InputPath,
			OutputFormat: request.Capability.TargetFormat,
			InputSize:    request.InputSize,
		}
		opts := queue.TaskOptions{
			MaxRetries: 0,
			Timeout:    time.Duration(request.Capability.ExecutionLimits.TimeoutSeconds) * time.Second,
		}

		if err := s.q.Enqueue(ctx, taskType, payload, opts); err != nil {
			errMsg := fmt.Sprintf("enqueue failed: %v", err)
			job.Status = domain.JobFailed
			job.Error = &errMsg
			completedAt := time.Now().UTC()
			job.CompletedAt = &completedAt
			_ = s.jobs.Update(ctx, job)
		}

		_ = s.audit.Create(ctx, &domain.AuditEvent{
			ID:        uuid.New(),
			EventType: domain.AuditJobCreated,
			FileID:    &request.FileID,
			JobID:     &job.ID,
			Details:   map[string]interface{}{"capabilityId": request.Capability.ID},
			CreatedAt: now,
		})
	}

	return jobs, nil
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
func (s *Service) RetryFailedJob(ctx context.Context, sourceJob *domain.Job, cap domain.Capability, inputPath string, inputSize int64) (*domain.Job, error) {
	return s.retryFailedJob(ctx, sourceJob, nil, cap, inputPath, inputSize)
}

func (s *Service) RetryFailedJobForGuest(ctx context.Context, guestSessionID uuid.UUID, sourceJob *domain.Job, cap domain.Capability, inputPath string, inputSize int64) (*domain.Job, error) {
	return s.retryFailedJob(ctx, sourceJob, &guestSessionID, cap, inputPath, inputSize)
}

func (s *Service) retryFailedJob(ctx context.Context, sourceJob *domain.Job, guestSessionID *uuid.UUID, cap domain.Capability, inputPath string, inputSize int64) (*domain.Job, error) {
	if sourceJob == nil {
		return nil, fmt.Errorf("source job is required")
	}
	if sourceJob.Status != domain.JobFailed {
		return nil, fmt.Errorf("%w: retry only allowed for failed jobs", domain.ErrInvalidTransition)
	}
	if sourceJob.AttemptNumber >= cap.ExecutionLimits.MaxRetries {
		return nil, fmt.Errorf("%w: retry attempt %d reached max retries %d", domain.ErrRetryLimitExceeded, sourceJob.AttemptNumber, cap.ExecutionLimits.MaxRetries)
	}

	sourceJobID := sourceJob.ID
	if sourceJob.SourceJobID != nil {
		sourceJobID = *sourceJob.SourceJobID
	}
	attemptNumber := sourceJob.AttemptNumber + 1

	var retryJob *domain.Job
	var err error
	if guestSessionID != nil {
		retryJob, err = s.createAndEnqueueWithAttempt(ctx, nil, guestSessionID, sourceJob.FileID, cap, inputPath, inputSize, &sourceJobID, attemptNumber)
	} else {
		retryJob, err = s.createAndEnqueueWithAttempt(ctx, sourceJob.UserID, nil, sourceJob.FileID, cap, inputPath, inputSize, &sourceJobID, attemptNumber)
	}
	if err != nil {
		return nil, err
	}

	_ = s.audit.Create(ctx, &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: domain.AuditJobRetried,
		FileID:    &sourceJob.FileID,
		JobID:     &retryJob.ID,
		Details: map[string]interface{}{
			"sourceJobId":     sourceJob.ID.String(),
			"rootSourceJobId": sourceJobID.String(),
			"attemptNumber":   attemptNumber,
			"maxRetries":      cap.ExecutionLimits.MaxRetries,
			"capabilityId":    sourceJob.CapabilityID,
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

	// Best-effort notification for terminal states.
	if s.notifier != nil {
		switch to {
		case domain.JobSucceeded:
			_ = s.notifier.NotifyJobCompleted(ctx, job)
		case domain.JobFailed:
			_ = s.notifier.NotifyJobFailed(ctx, job)
		}
	}

	return nil
}
