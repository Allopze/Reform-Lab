package email

import (
	"context"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/config"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/rs/zerolog"
)

// JobNotifier sends email notifications when jobs reach terminal states.
// It implements orchestrator.JobNotifier.
type JobNotifier struct {
	cfg    *config.Config
	email  *Service
	q      queue.JobQueue
	users  repository.UserRepository
	files  repository.FileRepository
	logger zerolog.Logger
}

// NewJobNotifier creates a notifier that enqueues emails on job completion/failure.
func NewJobNotifier(
	cfg *config.Config,
	email *Service,
	q queue.JobQueue,
	users repository.UserRepository,
	files repository.FileRepository,
	logger zerolog.Logger,
) *JobNotifier {
	return &JobNotifier{
		cfg:    cfg,
		email:  email,
		q:      q,
		users:  users,
		files:  files,
		logger: logger.With().Str("component", "email_notifier").Logger(),
	}
}

// NotifyJobCompleted enqueues a conversion-complete email.
func (n *JobNotifier) NotifyJobCompleted(ctx context.Context, job *domain.Job) error {
	return n.enqueueJobEmail(ctx, job, "conversion-complete", "")
}

// NotifyJobFailed enqueues a conversion-failed email.
func (n *JobNotifier) NotifyJobFailed(ctx context.Context, job *domain.Job) error {
	errMsg := "error desconocido"
	if job.Error != nil && *job.Error != "" {
		errMsg = *job.Error
	}
	return n.enqueueJobEmail(ctx, job, "conversion-failed", errMsg)
}

func (n *JobNotifier) enqueueJobEmail(ctx context.Context, job *domain.Job, templateKey, errMsg string) error {
	if !n.email.Configured(ctx) {
		return nil
	}
	if job.UserID == nil {
		return nil // guest jobs — no email to send
	}

	user, err := n.users.GetByID(ctx, *job.UserID)
	if err != nil || user == nil {
		n.logger.Warn().Err(err).Str("userId", job.UserID.String()).Msg("cannot look up user for email notification")
		return nil
	}

	file, err := n.files.GetByID(ctx, job.FileID)
	if err != nil || file == nil {
		n.logger.Warn().Err(err).Str("fileId", job.FileID.String()).Msg("cannot look up file for email notification")
		return nil
	}

	vars := map[string]string{
		"Name":         user.Name,
		"Email":        user.Email,
		"AppName":      "Reform Lab",
		"AppURL":       n.cfg.AppURL,
		"Year":         fmt.Sprintf("%d", time.Now().Year()),
		"FileName":     file.OriginalName,
		"OutputFormat": job.OutputFormat,
	}
	if errMsg != "" {
		vars["ErrorMessage"] = errMsg
	}

	err = n.q.EnqueueEmail(ctx, queue.EmailTaskPayload{
		TemplateKey: templateKey,
		To:          user.Email,
		Vars:        vars,
	}, queue.TaskOptions{MaxRetries: 3, Timeout: 30 * time.Second})

	if err != nil {
		n.logger.Warn().Err(err).
			Str("template", templateKey).
			Str("to", user.Email).
			Msg("failed to enqueue job notification email")
	}
	return err
}
