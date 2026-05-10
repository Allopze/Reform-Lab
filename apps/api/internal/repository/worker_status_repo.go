package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type WorkerFailure struct {
	ID       uuid.UUID  `json:"id"`
	WorkerID string     `json:"workerId"`
	TaskType *string    `json:"taskType,omitempty"`
	JobID    *uuid.UUID `json:"jobId,omitempty"`
	Error    string     `json:"error"`
	FailedAt time.Time  `json:"failedAt"`
}

type WorkerStatusSnapshot struct {
	ID                 string          `json:"id"`
	RuntimeMode        string          `json:"runtimeMode"`
	QueueMode          string          `json:"queueMode"`
	LastHeartbeatAt    time.Time       `json:"lastHeartbeatAt"`
	LastTaskType       *string         `json:"lastTaskType,omitempty"`
	LastJobID          *uuid.UUID      `json:"lastJobId,omitempty"`
	LastTaskStatus     string          `json:"lastTaskStatus"`
	LastTaskStartedAt  *time.Time      `json:"lastTaskStartedAt,omitempty"`
	LastTaskFinishedAt *time.Time      `json:"lastTaskFinishedAt,omitempty"`
	LastError          *string         `json:"lastError,omitempty"`
	Engines            map[string]bool `json:"engines"`
	RecentFailures     []WorkerFailure `json:"recentFailures"`
}

type WorkerStatusRepository interface {
	Heartbeat(ctx context.Context, snapshot WorkerStatusSnapshot) error
	RecordTaskStart(ctx context.Context, workerID, taskType string, jobID *uuid.UUID, startedAt time.Time) error
	RecordTaskFinish(ctx context.Context, workerID string, finishedAt time.Time, status string, errMsg *string) error
	List(ctx context.Context, recentFailuresPerWorker int) ([]WorkerStatusSnapshot, error)
	DeleteStaleBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type sqliteWorkerStatusRepo struct {
	db *sql.DB
}

func NewWorkerStatusRepository(db *sql.DB) WorkerStatusRepository {
	return &sqliteWorkerStatusRepo{db: db}
}

func (r *sqliteWorkerStatusRepo) Heartbeat(ctx context.Context, snapshot WorkerStatusSnapshot) error {
	if snapshot.Engines == nil {
		snapshot.Engines = map[string]bool{}
	}
	enginesJSON, err := json.Marshal(snapshot.Engines)
	if err != nil {
		return fmt.Errorf("marshal worker engines: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO worker_status (
			id, runtime_mode, queue_mode, last_heartbeat_at, last_task_type, last_job_id,
			last_task_status, last_task_started_at, last_task_finished_at, last_error, engines_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			runtime_mode = excluded.runtime_mode,
			queue_mode = excluded.queue_mode,
			last_heartbeat_at = excluded.last_heartbeat_at,
			engines_json = excluded.engines_json,
			updated_at = excluded.updated_at`,
		snapshot.ID,
		snapshot.RuntimeMode,
		snapshot.QueueMode,
		snapshot.LastHeartbeatAt.Format(timeLayout),
		nullableString(snapshot.LastTaskType),
		nullableUUIDString(snapshot.LastJobID),
		snapshot.LastTaskStatus,
		fmtTimePtr(snapshot.LastTaskStartedAt),
		fmtTimePtr(snapshot.LastTaskFinishedAt),
		nullableString(snapshot.LastError),
		string(enginesJSON),
		snapshot.LastHeartbeatAt.Format(timeLayout),
	)
	if err != nil {
		return fmt.Errorf("worker heartbeat: %w", err)
	}
	return nil
}

func (r *sqliteWorkerStatusRepo) RecordTaskStart(ctx context.Context, workerID, taskType string, jobID *uuid.UUID, startedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE worker_status
		 SET last_heartbeat_at = ?,
		     last_task_type = ?,
		     last_job_id = ?,
		     last_task_status = ?,
		     last_task_started_at = ?,
		     last_task_finished_at = NULL,
		     last_error = NULL,
		     updated_at = ?
		 WHERE id = ?`,
		startedAt.Format(timeLayout),
		taskType,
		nullableUUIDString(jobID),
		"running",
		startedAt.Format(timeLayout),
		startedAt.Format(timeLayout),
		workerID,
	)
	if err != nil {
		return fmt.Errorf("worker task start: %w", err)
	}
	return nil
}

func (r *sqliteWorkerStatusRepo) RecordTaskFinish(ctx context.Context, workerID string, finishedAt time.Time, status string, errMsg *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE worker_status
		 SET last_heartbeat_at = ?,
		     last_task_status = ?,
		     last_task_finished_at = ?,
		     last_error = ?,
		     updated_at = ?
		 WHERE id = ?`,
		finishedAt.Format(timeLayout),
		status,
		finishedAt.Format(timeLayout),
		nullableString(errMsg),
		finishedAt.Format(timeLayout),
		workerID,
	)
	if err != nil {
		return fmt.Errorf("worker task finish: %w", err)
	}
	if errMsg != nil {
		var taskType sql.NullString
		var jobIDStr sql.NullString
		if err := r.db.QueryRowContext(ctx, `SELECT last_task_type, last_job_id FROM worker_status WHERE id = ?`, workerID).Scan(&taskType, &jobIDStr); err == nil {
			failure := WorkerFailure{
				ID:       uuid.New(),
				WorkerID: workerID,
				Error:    *errMsg,
				FailedAt: finishedAt,
			}
			if taskType.Valid {
				failure.TaskType = &taskType.String
			}
			if jobIDStr.Valid && jobIDStr.String != "" {
				if parsed, parseErr := uuid.Parse(jobIDStr.String); parseErr == nil {
					failure.JobID = &parsed
				}
			}
			_, _ = r.db.ExecContext(ctx,
				`INSERT INTO worker_failures (id, worker_id, task_type, job_id, error, failed_at) VALUES (?, ?, ?, ?, ?, ?)`,
				failure.ID.String(),
				failure.WorkerID,
				nullableString(failure.TaskType),
				nullableUUIDString(failure.JobID),
				failure.Error,
				failure.FailedAt.Format(timeLayout),
			)
		}
	}
	return nil
}

func (r *sqliteWorkerStatusRepo) List(ctx context.Context, recentFailuresPerWorker int) ([]WorkerStatusSnapshot, error) {
	if recentFailuresPerWorker <= 0 {
		recentFailuresPerWorker = 3
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, runtime_mode, queue_mode, last_heartbeat_at, last_task_type, last_job_id,
		        last_task_status, last_task_started_at, last_task_finished_at, last_error, engines_json
		 FROM worker_status
		 ORDER BY updated_at DESC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list worker status: %w", err)
	}
	defer rows.Close()

	workers := make([]WorkerStatusSnapshot, 0)
	for rows.Next() {
		var item WorkerStatusSnapshot
		var heartbeatAt string
		var lastTaskType, lastJobID, lastTaskStartedAt, lastTaskFinishedAt, lastError sql.NullString
		var enginesRaw string
		if err := rows.Scan(
			&item.ID,
			&item.RuntimeMode,
			&item.QueueMode,
			&heartbeatAt,
			&lastTaskType,
			&lastJobID,
			&item.LastTaskStatus,
			&lastTaskStartedAt,
			&lastTaskFinishedAt,
			&lastError,
			&enginesRaw,
		); err != nil {
			return nil, fmt.Errorf("scan worker status: %w", err)
		}
		item.LastHeartbeatAt, _ = parseTime(heartbeatAt)
		if lastTaskType.Valid {
			item.LastTaskType = &lastTaskType.String
		}
		if lastJobID.Valid && lastJobID.String != "" {
			if parsed, parseErr := uuid.Parse(lastJobID.String); parseErr == nil {
				item.LastJobID = &parsed
			}
		}
		if lastTaskStartedAt.Valid {
			value := lastTaskStartedAt.String
			item.LastTaskStartedAt = timePtr(&value)
		}
		if lastTaskFinishedAt.Valid {
			value := lastTaskFinishedAt.String
			item.LastTaskFinishedAt = timePtr(&value)
		}
		if lastError.Valid {
			item.LastError = &lastError.String
		}
		item.Engines = map[string]bool{}
		if strings.TrimSpace(enginesRaw) != "" {
			_ = json.Unmarshal([]byte(enginesRaw), &item.Engines)
		}
		workers = append(workers, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range workers {
		failures, failureErr := r.listFailures(ctx, workers[index].ID, recentFailuresPerWorker)
		if failureErr != nil {
			return nil, failureErr
		}
		workers[index].RecentFailures = failures
	}
	return workers, nil
}

func (r *sqliteWorkerStatusRepo) listFailures(ctx context.Context, workerID string, limit int) ([]WorkerFailure, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, task_type, job_id, error, failed_at
		 FROM worker_failures
		 WHERE worker_id = ?
		 ORDER BY failed_at DESC
		 LIMIT ?`,
		workerID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list worker failures: %w", err)
	}
	defer rows.Close()
	items := make([]WorkerFailure, 0, limit)
	for rows.Next() {
		var item WorkerFailure
		var idStr, failedAt string
		var taskType sql.NullString
		var jobIDStr sql.NullString
		if err := rows.Scan(&idStr, &taskType, &jobIDStr, &item.Error, &failedAt); err != nil {
			return nil, fmt.Errorf("scan worker failure: %w", err)
		}
		item.ID, _ = uuid.Parse(idStr)
		item.WorkerID = workerID
		if taskType.Valid {
			item.TaskType = &taskType.String
		}
		if jobIDStr.Valid && jobIDStr.String != "" {
			if parsed, parseErr := uuid.Parse(jobIDStr.String); parseErr == nil {
				item.JobID = &parsed
			}
		}
		item.FailedAt, _ = parseTime(failedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *sqliteWorkerStatusRepo) DeleteStaleBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM worker_status WHERE last_heartbeat_at < ?`, cutoff.Format(timeLayout))
	if err != nil {
		return 0, fmt.Errorf("delete stale worker status: %w", err)
	}
	return res.RowsAffected()
}
