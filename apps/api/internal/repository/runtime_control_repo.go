package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

type RuntimeControlState struct {
	JobIntakePaused bool       `json:"jobIntakePaused"`
	PauseReason     *string    `json:"pauseReason,omitempty"`
	UpdatedBy       *uuid.UUID `json:"updatedBy,omitempty"`
	UpdatedAt       string     `json:"updatedAt"`
}

type RuntimeControlRepository interface {
	Get(ctx context.Context) (*RuntimeControlState, error)
	SetJobIntakePaused(ctx context.Context, paused bool, reason *string, updatedBy *uuid.UUID) error
}

type sqliteRuntimeControlRepo struct {
	db *sql.DB
}

func NewRuntimeControlRepository(db *sql.DB) RuntimeControlRepository {
	return &sqliteRuntimeControlRepo{db: db}
}

func (r *sqliteRuntimeControlRepo) Get(ctx context.Context) (*RuntimeControlState, error) {
	var state RuntimeControlState
	var pausedInt int
	var pauseReason, updatedBy sql.NullString
	if err := r.db.QueryRowContext(ctx,
		`SELECT job_intake_paused, pause_reason, updated_by, updated_at FROM runtime_controls WHERE id = 1`,
	).Scan(&pausedInt, &pauseReason, &updatedBy, &state.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get runtime controls: %w", err)
	}
	state.JobIntakePaused = pausedInt == 1
	if pauseReason.Valid {
		state.PauseReason = &pauseReason.String
	}
	if updatedBy.Valid && updatedBy.String != "" {
		if parsed, parseErr := uuid.Parse(updatedBy.String); parseErr == nil {
			state.UpdatedBy = &parsed
		}
	}
	return &state, nil
}

func (r *sqliteRuntimeControlRepo) SetJobIntakePaused(ctx context.Context, paused bool, reason *string, updatedBy *uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE runtime_controls
		 SET job_intake_paused = ?, pause_reason = ?, updated_by = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
		 WHERE id = 1`,
		paused,
		nullableString(reason),
		nullableUUIDString(updatedBy),
	)
	if err != nil {
		return fmt.Errorf("update runtime controls: %w", err)
	}
	return nil
}
