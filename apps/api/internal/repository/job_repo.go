package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// JobRepository persists and retrieves Job records.
type JobRepository interface {
	Create(ctx context.Context, j *domain.Job) error
	CreateIfUnderLimit(ctx context.Context, j *domain.Job, maxActive int) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error)
	Update(ctx context.Context, j *domain.Job) error
	CountActiveByUser(ctx context.Context, userID uuid.UUID) (int, error)
	CountActiveByGuestSession(ctx context.Context, sessionID uuid.UUID) (int, error)
	ExpireArtifact(ctx context.Context, artifactID uuid.UUID, expiredAt time.Time) error
}

type sqliteJobRepo struct {
	db *sql.DB
}

// NewJobRepository creates a SQLite-backed JobRepository.
func NewJobRepository(db *sql.DB) JobRepository {
	return &sqliteJobRepo{db: db}
}

func (r *sqliteJobRepo) Create(ctx context.Context, j *domain.Job) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO jobs (id, user_id, file_id, capability_id, output_format, status, progress, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID.String(), nullableUUIDString(j.UserID), j.FileID.String(), j.CapabilityID, j.OutputFormat,
		string(j.Status), j.Progress, j.CreatedAt.Format(timeLayout),
	)
	return err
}

// CreateIfUnderLimit atomically checks the active job count for the user and
// inserts the job only if the count is below maxActive. Returns
// domain.ErrTooManyActiveJobs if the limit would be exceeded.
func (r *sqliteJobRepo) CreateIfUnderLimit(ctx context.Context, j *domain.Job, maxActive int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if j.UserID != nil && maxActive > 0 {
		var count int
		err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM jobs WHERE user_id = ? AND status IN (?, ?)`,
			j.UserID.String(), string(domain.JobQueued), string(domain.JobRunning),
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("count active jobs: %w", err)
		}
		if count >= maxActive {
			return domain.ErrTooManyActiveJobs
		}
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO jobs (id, user_id, file_id, capability_id, output_format, status, progress, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID.String(), nullableUUIDString(j.UserID), j.FileID.String(), j.CapabilityID, j.OutputFormat,
		string(j.Status), j.Progress, j.CreatedAt.Format(timeLayout),
	)
	if err != nil {
		return fmt.Errorf("insert job: %w", err)
	}

	return tx.Commit()
}

func (r *sqliteJobRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error) {
	var j domain.Job
	var status, idStr, fileIDStr, createdAt string
	var userIDStr *string
	var errMsg, artifactIDStr, startedAt, completedAt *string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, file_id, capability_id, output_format, status, progress, error, artifact_id, started_at, completed_at, created_at
		 FROM jobs WHERE id = ?`, id.String(),
	).Scan(
		&idStr, &userIDStr, &fileIDStr, &j.CapabilityID, &j.OutputFormat,
		&status, &j.Progress, &errMsg, &artifactIDStr,
		&startedAt, &completedAt, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get job %s: %w", id, err)
	}

	j.ID = id
	if userIDStr != nil && *userIDStr != "" {
		uid, parseErr := uuid.Parse(*userIDStr)
		if parseErr == nil {
			j.UserID = &uid
		}
	}
	j.FileID, _ = uuid.Parse(fileIDStr)
	j.Status = domain.JobStatus(status)
	j.Error = errMsg
	if artifactIDStr != nil && *artifactIDStr != "" {
		aid, _ := uuid.Parse(*artifactIDStr)
		j.ArtifactID = &aid
	}
	j.StartedAt = timePtr(startedAt)
	j.CompletedAt = timePtr(completedAt)
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse job created_at: %w", err)
	}
	j.CreatedAt = t
	return &j, nil
}

func (r *sqliteJobRepo) Update(ctx context.Context, j *domain.Job) error {
	var artifactIDStr *string
	if j.ArtifactID != nil {
		s := j.ArtifactID.String()
		artifactIDStr = &s
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs SET status=?, progress=?, error=?, artifact_id=?, started_at=?, completed_at=?
		 WHERE id=?`,
		string(j.Status), j.Progress, j.Error, artifactIDStr,
		fmtTimePtr(j.StartedAt), fmtTimePtr(j.CompletedAt), j.ID.String(),
	)
	return err
}

func (r *sqliteJobRepo) CountActiveByUser(ctx context.Context, userID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM jobs
		 WHERE user_id = ? AND status IN (?, ?)`,
		userID.String(), string(domain.JobQueued), string(domain.JobRunning),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active jobs for user %s: %w", userID, err)
	}
	return count, nil
}

func (r *sqliteJobRepo) CountActiveByGuestSession(ctx context.Context, sessionID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*)
		 FROM jobs j
		 JOIN files f ON f.id = j.file_id
		 WHERE f.guest_session_id = ?
		   AND f.expired_at IS NULL
		   AND j.status IN (?, ?)`,
		sessionID.String(), string(domain.JobQueued), string(domain.JobRunning),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active jobs for guest session %s: %w", sessionID, err)
	}
	return count, nil
}

func (r *sqliteJobRepo) ExpireArtifact(ctx context.Context, artifactID uuid.UUID, expiredAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE jobs
		 SET status = ?, artifact_id = NULL, completed_at = COALESCE(completed_at, ?)
		 WHERE artifact_id = ?`,
		string(domain.JobExpired), expiredAt.Format(timeLayout), artifactID.String(),
	)
	return err
}
