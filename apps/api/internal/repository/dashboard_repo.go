package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

type UserDashboardJob struct {
	JobID            uuid.UUID        `json:"jobId"`
	FileID           uuid.UUID        `json:"fileId"`
	FileName         string           `json:"fileName"`
	DetectedFamily   string           `json:"detectedFamily"`
	CapabilityID     string           `json:"capabilityId"`
	OutputFormat     string           `json:"outputFormat"`
	Status           domain.JobStatus `json:"status"`
	Progress         int              `json:"progress"`
	Error            *string          `json:"error,omitempty"`
	ArtifactID       *uuid.UUID       `json:"artifactId,omitempty"`
	ArtifactFileName *string          `json:"artifactFileName,omitempty"`
	ExpiresAt        *time.Time       `json:"expiresAt,omitempty"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

type UserDashboardData struct {
	TotalFiles    int                `json:"totalFiles"`
	TotalJobs     int                `json:"totalJobs"`
	ActiveJobs    int                `json:"activeJobs"`
	SucceededJobs int                `json:"succeededJobs"`
	FailedJobs    int                `json:"failedJobs"`
	RecentJobs    []UserDashboardJob `json:"recentJobs"`
}

type AdminDashboardJob struct {
	JobID        uuid.UUID        `json:"jobId"`
	UserName     string           `json:"userName"`
	UserEmail    string           `json:"userEmail"`
	FileName     string           `json:"fileName"`
	CapabilityID string           `json:"capabilityId"`
	OutputFormat string           `json:"outputFormat"`
	Status       domain.JobStatus `json:"status"`
	Error        *string          `json:"error,omitempty"`
	UpdatedAt    time.Time        `json:"updatedAt"`
}

type AdminUsageStat struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

type AdminDashboardData struct {
	TotalUsers         int                 `json:"totalUsers"`
	TotalFiles         int                 `json:"totalFiles"`
	TotalJobs          int                 `json:"totalJobs"`
	QueuedJobs         int                 `json:"queuedJobs"`
	RunningJobs        int                 `json:"runningJobs"`
	SucceededJobs      int                 `json:"succeededJobs"`
	FailedJobs         int                 `json:"failedJobs"`
	CancelledJobs      int                 `json:"cancelledJobs"`
	SuccessRatePct     float64             `json:"successRatePct"`
	AverageDurationSec float64             `json:"averageDurationSec"`
	AvailableEngines   int                 `json:"availableEngines"`
	TotalEngines       int                 `json:"totalEngines"`
	UnavailableEngines []string            `json:"unavailableEngines"`
	EngineUsage        []AdminUsageStat    `json:"engineUsage"`
	RecentAudit        []domain.AuditEvent `json:"recentAudit"`
	RecentJobs         []AdminDashboardJob `json:"recentJobs"`
	CapabilityUsage    []AdminUsageStat    `json:"-"`
}

type DashboardRepository interface {
	GetUserDashboard(ctx context.Context, userID uuid.UUID, limit int) (*UserDashboardData, error)
	GetAdminDashboard(ctx context.Context, limit int) (*AdminDashboardData, error)
}

type sqliteDashboardRepo struct {
	db *sql.DB
}

func NewDashboardRepository(db *sql.DB) DashboardRepository {
	return &sqliteDashboardRepo{db: db}
}

func (r *sqliteDashboardRepo) GetUserDashboard(ctx context.Context, userID uuid.UUID, limit int) (*UserDashboardData, error) {
	result := &UserDashboardData{}
	ownerID := userID.String()

	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM files WHERE user_id = ?`, ownerID).Scan(&result.TotalFiles); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE user_id = ?`, ownerID).Scan(&result.TotalJobs); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE user_id = ? AND status IN (?, ?)`, ownerID, string(domain.JobQueued), string(domain.JobRunning)).Scan(&result.ActiveJobs); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE user_id = ? AND status = ?`, ownerID, string(domain.JobSucceeded)).Scan(&result.SucceededJobs); err != nil {
		return nil, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE user_id = ? AND status = ?`, ownerID, string(domain.JobFailed)).Scan(&result.FailedJobs); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT j.id, j.file_id, f.original_name, f.format_family, j.capability_id, j.output_format, j.status, j.progress, j.error,
		        j.artifact_id, a.file_name, a.expires_at,
		        COALESCE(j.completed_at, j.started_at, j.created_at)
		 FROM jobs j
		 JOIN files f ON f.id = j.file_id
		 LEFT JOIN artifacts a ON a.id = j.artifact_id
		 WHERE j.user_id = ?
		 ORDER BY j.created_at DESC
		 LIMIT ?`,
		ownerID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result.RecentJobs = make([]UserDashboardJob, 0)
	for rows.Next() {
		var item UserDashboardJob
		var jobIDStr, fileIDStr, status, updatedAt string
		var artifactIDStr, artifactFileName, expiresAt *string

		if err := rows.Scan(
			&jobIDStr, &fileIDStr, &item.FileName, &item.DetectedFamily, &item.CapabilityID, &item.OutputFormat,
			&status, &item.Progress, &item.Error, &artifactIDStr, &artifactFileName, &expiresAt, &updatedAt,
		); err != nil {
			return nil, err
		}

		item.JobID, _ = uuid.Parse(jobIDStr)
		item.FileID, _ = uuid.Parse(fileIDStr)
		item.Status = domain.JobStatus(status)
		if artifactIDStr != nil && *artifactIDStr != "" {
			artifactID, parseErr := uuid.Parse(*artifactIDStr)
			if parseErr == nil {
				item.ArtifactID = &artifactID
			}
		}
		item.ArtifactFileName = artifactFileName
		item.ExpiresAt = timePtr(expiresAt)
		parsedUpdatedAt, parseErr := parseTime(updatedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse dashboard updated_at: %w", parseErr)
		}
		item.UpdatedAt = parsedUpdatedAt
		result.RecentJobs = append(result.RecentJobs, item)
	}

	return result, rows.Err()
}

func (r *sqliteDashboardRepo) GetAdminDashboard(ctx context.Context, limit int) (*AdminDashboardData, error) {
	result := &AdminDashboardData{}

	for _, query := range []struct {
		target *int
		sql    string
		args   []interface{}
	}{
		{target: &result.TotalUsers, sql: `SELECT COUNT(*) FROM users`},
		{target: &result.TotalFiles, sql: `SELECT COUNT(*) FROM files`},
		{target: &result.TotalJobs, sql: `SELECT COUNT(*) FROM jobs`},
		{target: &result.QueuedJobs, sql: `SELECT COUNT(*) FROM jobs WHERE status = ?`, args: []interface{}{string(domain.JobQueued)}},
		{target: &result.RunningJobs, sql: `SELECT COUNT(*) FROM jobs WHERE status = ?`, args: []interface{}{string(domain.JobRunning)}},
		{target: &result.SucceededJobs, sql: `SELECT COUNT(*) FROM jobs WHERE status = ?`, args: []interface{}{string(domain.JobSucceeded)}},
		{target: &result.FailedJobs, sql: `SELECT COUNT(*) FROM jobs WHERE status = ?`, args: []interface{}{string(domain.JobFailed)}},
		{target: &result.CancelledJobs, sql: `SELECT COUNT(*) FROM jobs WHERE status = ?`, args: []interface{}{string(domain.JobCancelled)}},
	} {
		if err := r.db.QueryRowContext(ctx, query.sql, query.args...).Scan(query.target); err != nil {
			return nil, err
		}
	}

	terminalJobs := result.SucceededJobs + result.FailedJobs + result.CancelledJobs
	if terminalJobs > 0 {
		result.SuccessRatePct = (float64(result.SucceededJobs) / float64(terminalJobs)) * 100
	}
	if err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(AVG((julianday(completed_at) - julianday(started_at)) * 86400.0), 0)
		 FROM jobs
		 WHERE started_at IS NOT NULL
		   AND completed_at IS NOT NULL
		   AND status IN (?, ?, ?)`,
		string(domain.JobSucceeded), string(domain.JobFailed), string(domain.JobCancelled),
	).Scan(&result.AverageDurationSec); err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT j.id, COALESCE(u.name, 'Sin propietario'), COALESCE(u.email, 'sin-propietario@local'),
		        COALESCE(f.original_name, 'archivo-desconocido'), j.capability_id, j.output_format, j.status, j.error,
		        COALESCE(j.completed_at, j.started_at, j.created_at)
		 FROM jobs j
		 LEFT JOIN users u ON u.id = j.user_id
		 LEFT JOIN files f ON f.id = j.file_id
		 ORDER BY j.created_at DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result.RecentJobs = make([]AdminDashboardJob, 0)
	for rows.Next() {
		var item AdminDashboardJob
		var jobIDStr, status, updatedAt string

		if err := rows.Scan(
			&jobIDStr, &item.UserName, &item.UserEmail, &item.FileName,
			&item.CapabilityID, &item.OutputFormat, &status, &item.Error, &updatedAt,
		); err != nil {
			return nil, err
		}

		item.JobID, _ = uuid.Parse(jobIDStr)
		item.Status = domain.JobStatus(status)
		parsedUpdatedAt, parseErr := parseTime(updatedAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse admin dashboard updated_at: %w", parseErr)
		}
		item.UpdatedAt = parsedUpdatedAt
		result.RecentJobs = append(result.RecentJobs, item)
	}

	usageRows, err := r.db.QueryContext(ctx,
		`SELECT capability_id, COUNT(*)
		 FROM jobs
		 GROUP BY capability_id
		 ORDER BY COUNT(*) DESC, capability_id ASC
		 LIMIT 8`,
	)
	if err != nil {
		return nil, err
	}
	defer usageRows.Close()

	result.CapabilityUsage = make([]AdminUsageStat, 0)
	for usageRows.Next() {
		var stat AdminUsageStat
		if err := usageRows.Scan(&stat.Key, &stat.Count); err != nil {
			return nil, err
		}
		result.CapabilityUsage = append(result.CapabilityUsage, stat)
	}
	if err := usageRows.Err(); err != nil {
		return nil, err
	}

	return result, rows.Err()
}
