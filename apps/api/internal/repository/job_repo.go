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
	ListIDsForAdmin(ctx context.Context, filter AdminJobFilter) ([]uuid.UUID, error)
	QueueHistory(ctx context.Context) ([]AdminQueueHistoryPoint, error)
}

// AdminJobFilter defines filters for the admin jobs listing.
type AdminJobFilter struct {
	Status       string // empty = all
	CapabilityID string // empty = all
	Search       string // free text match against user name, email, file name
	StalledOnly  bool   // true = only jobs considered stalled
	Limit        int    // page size (max 100, default 50)
	Offset       int
}

// AdminJobRow is a single row in the admin job listing.
type AdminJobRow struct {
	JobID         uuid.UUID        `json:"jobId"`
	UserName      string           `json:"userName"`
	UserEmail     string           `json:"userEmail"`
	FileName      string           `json:"fileName"`
	CapabilityID  string           `json:"capabilityId"`
	OutputFormat  string           `json:"outputFormat"`
	Status        domain.JobStatus `json:"status"`
	Error         *string          `json:"error,omitempty"`
	CreatedAt     time.Time        `json:"createdAt"`
	UpdatedAt     time.Time        `json:"updatedAt"`
	Stalled       bool             `json:"stalled"`
	StalledReason *string          `json:"stalledReason,omitempty"`
	BacklogAgeSec *int64           `json:"backlogAgeSec,omitempty"`
}

// AdminJobPage is a paginated result for admin job listing.
type AdminJobPage struct {
	Jobs               []AdminJobRow `json:"jobs"`
	Total              int           `json:"total"`
	StalledJobs        int           `json:"stalledJobs"`
	StalledQueuedJobs  int           `json:"stalledQueuedJobs"`
	StalledRunningJobs int           `json:"stalledRunningJobs"`
}

type AdminQueueHistoryPoint struct {
	Window         string  `json:"window"`
	EnqueuedJobs   int     `json:"enqueuedJobs"`
	FailedJobs     int     `json:"failedJobs"`
	CompletedJobs  int     `json:"completedJobs"`
	AverageLatency float64 `json:"averageLatencySec"`
}

const (
	adminQueuedStalledAfter  = 15 * time.Minute
	adminRunningStalledAfter = 30 * time.Minute
)

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
	now := time.Now().UTC()

	baseWhere := "1=1"
	baseArgs := []interface{}{}

	if filter.Status != "" {
		baseWhere += " AND j.status = ?"
		baseArgs = append(baseArgs, filter.Status)
	}
	if filter.CapabilityID != "" {
		baseWhere += " AND j.capability_id = ?"
		baseArgs = append(baseArgs, filter.CapabilityID)
	}
	if filter.Search != "" {
		baseWhere += " AND (u.name LIKE ? OR u.email LIKE ? OR f.original_name LIKE ?)"
		like := "%" + filter.Search + "%"
		baseArgs = append(baseArgs, like, like, like)
	}

	where := baseWhere
	args := append([]interface{}{}, baseArgs...)
	if filter.StalledOnly {
		where += " AND (" + adminStalledWhereClause() + ")"
		args = append(args, adminStalledWhereArgs(now)...)
	}

	stalledQueued, stalledRunning, err := r.countStalledAdminJobs(ctx, baseWhere, baseArgs, now)
	if err != nil {
		return nil, err
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
		applyAdminStalledSignals(&row, now)
		jobs = append(jobs, row)
	}

	return &AdminJobPage{
		Jobs:               jobs,
		Total:              total,
		StalledJobs:        stalledQueued + stalledRunning,
		StalledQueuedJobs:  stalledQueued,
		StalledRunningJobs: stalledRunning,
	}, nil
}

func (r *sqliteJobRepo) ListIDsForAdmin(ctx context.Context, filter AdminJobFilter) ([]uuid.UUID, error) {
	now := time.Now().UTC()
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
	if filter.StalledOnly {
		where += " AND (" + adminStalledWhereClause() + ")"
		args = append(args, adminStalledWhereArgs(now)...)
	}
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT j.id
		 FROM jobs j
		 LEFT JOIN users u ON u.id = j.user_id
		 LEFT JOIN files f ON f.id = j.file_id
		 WHERE %s
		 ORDER BY j.created_at DESC`, where),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("list admin job ids: %w", err)
	}
	defer rows.Close()
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan admin job id: %w", err)
		}
		if parsed, parseErr := uuid.Parse(raw); parseErr == nil {
			ids = append(ids, parsed)
		}
	}
	return ids, rows.Err()
}

func (r *sqliteJobRepo) QueueHistory(ctx context.Context) ([]AdminQueueHistoryPoint, error) {
	windows := []struct {
		label string
		since time.Duration
	}{
		{label: "5m", since: 5 * time.Minute},
		{label: "15m", since: 15 * time.Minute},
		{label: "1h", since: time.Hour},
	}
	points := make([]AdminQueueHistoryPoint, 0, len(windows))
	for _, window := range windows {
		point := AdminQueueHistoryPoint{Window: window.label}
		since := time.Now().UTC().Add(-window.since).Format(timeLayout)
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE created_at >= ?`, since).Scan(&point.EnqueuedJobs); err != nil {
			return nil, fmt.Errorf("count enqueued jobs history: %w", err)
		}
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE status = ? AND completed_at >= ?`, string(domain.JobFailed), since).Scan(&point.FailedJobs); err != nil {
			return nil, fmt.Errorf("count failed jobs history: %w", err)
		}
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE status IN (?, ?, ?) AND completed_at >= ?`, string(domain.JobSucceeded), string(domain.JobFailed), string(domain.JobCancelled), since).Scan(&point.CompletedJobs); err != nil {
			return nil, fmt.Errorf("count completed jobs history: %w", err)
		}
		if err := r.db.QueryRowContext(ctx,
			`SELECT COALESCE(AVG((julianday(completed_at) - julianday(started_at)) * 86400.0), 0)
			 FROM jobs
			 WHERE started_at IS NOT NULL
			   AND completed_at IS NOT NULL
			   AND completed_at >= ?`,
			since,
		).Scan(&point.AverageLatency); err != nil {
			return nil, fmt.Errorf("avg latency history: %w", err)
		}
		points = append(points, point)
	}
	return points, nil
}

func (r *sqliteJobRepo) countStalledAdminJobs(ctx context.Context, where string, args []interface{}, now time.Time) (int, int, error) {
	query := fmt.Sprintf(
		`SELECT
			COALESCE(SUM(CASE WHEN j.status = ? AND julianday(j.created_at) <= julianday(?) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN j.status = ? AND julianday(COALESCE(j.started_at, j.created_at)) <= julianday(?) THEN 1 ELSE 0 END), 0)
		 FROM jobs j
		 LEFT JOIN users u ON u.id = j.user_id
		 LEFT JOIN files f ON f.id = j.file_id
		 WHERE %s`, where)

	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(
		queryArgs,
		string(domain.JobQueued), now.Add(-adminQueuedStalledAfter).Format(timeLayout),
		string(domain.JobRunning), now.Add(-adminRunningStalledAfter).Format(timeLayout),
	)

	var stalledQueued int
	var stalledRunning int
	if err := r.db.QueryRowContext(ctx, query, queryArgs...).Scan(&stalledQueued, &stalledRunning); err != nil {
		return 0, 0, fmt.Errorf("count stalled admin jobs: %w", err)
	}

	return stalledQueued, stalledRunning, nil
}

func adminStalledWhereClause() string {
	return `j.status = ? AND julianday(j.created_at) <= julianday(?)
		OR j.status = ? AND julianday(COALESCE(j.started_at, j.created_at)) <= julianday(?)`
}

func adminStalledWhereArgs(now time.Time) []interface{} {
	return []interface{}{
		string(domain.JobQueued), now.Add(-adminQueuedStalledAfter).Format(timeLayout),
		string(domain.JobRunning), now.Add(-adminRunningStalledAfter).Format(timeLayout),
	}
}

func applyAdminStalledSignals(row *AdminJobRow, now time.Time) {
	var reference time.Time
	var threshold time.Duration
	var reason string

	switch row.Status {
	case domain.JobQueued:
		reference = row.CreatedAt
		threshold = adminQueuedStalledAfter
		reason = "queued_too_long"
	case domain.JobRunning:
		reference = row.UpdatedAt
		if reference.IsZero() {
			reference = row.CreatedAt
		}
		threshold = adminRunningStalledAfter
		reason = "running_too_long"
	default:
		return
	}

	ageSec := int64(now.Sub(reference).Seconds())
	if ageSec < 0 {
		ageSec = 0
	}
	row.BacklogAgeSec = &ageSec

	if time.Duration(ageSec)*time.Second >= threshold {
		row.Stalled = true
		row.StalledReason = &reason
	}
}
