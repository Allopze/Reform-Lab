package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// JobRepository persists and retrieves Job records.
type JobRepository interface {
	Create(ctx context.Context, j *domain.Job) error
	CreateIfUnderLimit(ctx context.Context, j *domain.Job, maxActive int) error
	CreateManyIfUnderLimit(ctx context.Context, jobs []*domain.Job, maxActive int) error
	CreateIfUnderGuestLimit(ctx context.Context, sessionID uuid.UUID, j *domain.Job, maxActive int) error
	CreateManyIfUnderGuestLimit(ctx context.Context, sessionID uuid.UUID, jobs []*domain.Job, maxActive int) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Job, error)
	Update(ctx context.Context, j *domain.Job) error
	CountByStatuses(ctx context.Context, statuses ...domain.JobStatus) (int, error)
	CountActiveByUser(ctx context.Context, userID uuid.UUID) (int, error)
	CountActiveByGuestSession(ctx context.Context, sessionID uuid.UUID) (int, error)
	ExpireArtifact(ctx context.Context, artifactID uuid.UUID, expiredAt time.Time) error
	ListForAdmin(ctx context.Context, filter AdminJobFilter) (*AdminJobPage, error)
}

// AdminJobFilter defines filters for the admin jobs listing.
type AdminJobFilter struct {
	Status       string // empty = all
	CapabilityID string // empty = all
	Search       string // free text match against user name, email, file name
	Limit        int    // page size (max 100, default 50)
	Offset       int
}

// AdminJobRow is a single row in the admin job listing.
type AdminJobRow struct {
	JobID        uuid.UUID        `json:"jobId"`
	UserName     string           `json:"userName"`
	UserEmail    string           `json:"userEmail"`
	FileName     string           `json:"fileName"`
	CapabilityID string           `json:"capabilityId"`
	OutputFormat string           `json:"outputFormat"`
	Status       domain.JobStatus `json:"status"`
	Error        *string          `json:"error,omitempty"`
	CreatedAt    time.Time        `json:"createdAt"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

// AdminJobPage is a paginated result for admin job listing.
type AdminJobPage struct {
	Jobs  []AdminJobRow `json:"jobs"`
	Total int           `json:"total"`
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
	return r.createManyIfUnderLimit(ctx, []*domain.Job{j}, maxActive, nil)
}

func (r *sqliteJobRepo) CreateManyIfUnderLimit(ctx context.Context, jobs []*domain.Job, maxActive int) error {
	return r.createManyIfUnderLimit(ctx, jobs, maxActive, nil)
}

func (r *sqliteJobRepo) CreateIfUnderGuestLimit(ctx context.Context, sessionID uuid.UUID, j *domain.Job, maxActive int) error {
	return r.createManyIfUnderLimit(ctx, []*domain.Job{j}, maxActive, &sessionID)
}

func (r *sqliteJobRepo) CreateManyIfUnderGuestLimit(ctx context.Context, sessionID uuid.UUID, jobs []*domain.Job, maxActive int) error {
	return r.createManyIfUnderLimit(ctx, jobs, maxActive, &sessionID)
}

func (r *sqliteJobRepo) createManyIfUnderLimit(ctx context.Context, jobs []*domain.Job, maxActive int, guestSessionID *uuid.UUID) error {
	if len(jobs) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if jobs[0].UserID != nil && maxActive > 0 {
		var count int
		err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM jobs WHERE user_id = ? AND status IN (?, ?)`,
			jobs[0].UserID.String(), string(domain.JobQueued), string(domain.JobRunning),
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("count active jobs: %w", err)
		}
		if count+len(jobs) > maxActive {
			return domain.ErrTooManyActiveJobs
		}
	} else if guestSessionID != nil && maxActive > 0 {
		var count int
		err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*)
			 FROM jobs j
			 JOIN files f ON f.id = j.file_id
			 WHERE f.guest_session_id = ?
			   AND f.expired_at IS NULL
			   AND j.status IN (?, ?)`,
			guestSessionID.String(), string(domain.JobQueued), string(domain.JobRunning),
		).Scan(&count)
		if err != nil {
			return fmt.Errorf("count active guest jobs: %w", err)
		}
		if count+len(jobs) > maxActive {
			return domain.ErrTooManyActiveJobs
		}
	}

	for _, job := range jobs {
		_, err = tx.ExecContext(ctx,
			`INSERT INTO jobs (id, user_id, file_id, capability_id, output_format, status, progress, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			job.ID.String(), nullableUUIDString(job.UserID), job.FileID.String(), job.CapabilityID, job.OutputFormat,
			string(job.Status), job.Progress, job.CreatedAt.Format(timeLayout),
		)
		if err != nil {
			return fmt.Errorf("insert job: %w", err)
		}
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

func (r *sqliteJobRepo) CountByStatuses(ctx context.Context, statuses ...domain.JobStatus) (int, error) {
	if len(statuses) == 0 {
		return 0, nil
	}

	placeholders := make([]string, 0, len(statuses))
	args := make([]interface{}, 0, len(statuses))
	for _, status := range statuses {
		placeholders = append(placeholders, "?")
		args = append(args, string(status))
	}

	query := fmt.Sprintf(
		`SELECT COUNT(*) FROM jobs WHERE status IN (%s)`,
		strings.Join(placeholders, ", "),
	)

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count jobs by status: %w", err)
	}

	return count, nil
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

func (r *sqliteJobRepo) ListForAdmin(ctx context.Context, filter AdminJobFilter) (*AdminJobPage, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	where := "1=1"
	args := []interface{}{}

	if filter.Status != "" {
		where += " AND j.status = ?"
		args = append(args, filter.Status)
	}
	if filter.CapabilityID != "" {
		where += " AND j.capability_id = ?"
		args = append(args, filter.CapabilityID)
	}
	if filter.Search != "" {
		where += " AND (u.name LIKE ? OR u.email LIKE ? OR f.original_name LIKE ?)"
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like)
	}

	// Count total matching rows.
	var total int
	countQuery := fmt.Sprintf(
		`SELECT COUNT(*) FROM jobs j
		 LEFT JOIN users u ON u.id = j.user_id
		 LEFT JOIN files f ON f.id = j.file_id
		 WHERE %s`, where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count admin jobs: %w", err)
	}

	// Fetch page.
	dataQuery := fmt.Sprintf(
		`SELECT j.id, COALESCE(u.name, 'Sin propietario'), COALESCE(u.email, 'sin-propietario@local'),
		        COALESCE(f.original_name, 'archivo-desconocido'), j.capability_id, j.output_format,
		        j.status, j.error, j.created_at,
		        COALESCE(j.completed_at, j.started_at, j.created_at)
		 FROM jobs j
		 LEFT JOIN users u ON u.id = j.user_id
		 LEFT JOIN files f ON f.id = j.file_id
		 WHERE %s
		 ORDER BY j.created_at DESC
		 LIMIT ? OFFSET ?`, where)
	pageArgs := append(args, filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, dataQuery, pageArgs...)
	if err != nil {
		return nil, fmt.Errorf("list admin jobs: %w", err)
	}
	defer rows.Close()

	jobs := make([]AdminJobRow, 0, filter.Limit)
	for rows.Next() {
		var row AdminJobRow
		var jobIDStr, status, createdAt, updatedAt string
		if err := rows.Scan(
			&jobIDStr, &row.UserName, &row.UserEmail, &row.FileName,
			&row.CapabilityID, &row.OutputFormat, &status, &row.Error,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan admin job row: %w", err)
		}
		row.JobID, _ = uuid.Parse(jobIDStr)
		row.Status = domain.JobStatus(status)
		row.CreatedAt, _ = parseTime(createdAt)
		row.UpdatedAt, _ = parseTime(updatedAt)
		jobs = append(jobs, row)
	}

	return &AdminJobPage{Jobs: jobs, Total: total}, nil
}
