package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// AuditRepository persists audit events.
type AuditRepository interface {
	Create(ctx context.Context, e *domain.AuditEvent) error
	ListRecent(ctx context.Context, limit int, eventType *domain.AuditEventType) ([]domain.AuditEvent, error)
}

type sqliteAuditRepo struct {
	db *sql.DB
}

// NewAuditRepository creates a SQLite-backed AuditRepository.
func NewAuditRepository(db *sql.DB) AuditRepository {
	return &sqliteAuditRepo{db: db}
}

func (r *sqliteAuditRepo) Create(ctx context.Context, e *domain.AuditEvent) error {
	details, err := json.Marshal(e.Details)
	if err != nil {
		return fmt.Errorf("marshal audit details: %w", err)
	}

	var fileID, jobID *string
	if e.FileID != nil {
		s := e.FileID.String()
		fileID = &s
	}
	if e.JobID != nil {
		s := e.JobID.String()
		jobID = &s
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO audit_events (id, event_type, file_id, job_id, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID.String(), string(e.EventType), fileID, jobID, string(details), e.CreatedAt.Format(timeLayout),
	)
	return err
}

func (r *sqliteAuditRepo) ListRecent(ctx context.Context, limit int, eventType *domain.AuditEventType) ([]domain.AuditEvent, error) {
	query := `SELECT id, event_type, file_id, job_id, details, created_at
		 FROM audit_events`
	args := make([]interface{}, 0, 2)
	if eventType != nil {
		query += ` WHERE event_type = ?`
		args = append(args, string(*eventType))
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]domain.AuditEvent, 0)
	for rows.Next() {
		var event domain.AuditEvent
		var idStr, eventTypeStr, detailsStr, createdAt string
		var fileIDStr, jobIDStr *string

		if err := rows.Scan(&idStr, &eventTypeStr, &fileIDStr, &jobIDStr, &detailsStr, &createdAt); err != nil {
			return nil, err
		}

		event.ID, _ = uuid.Parse(idStr)
		event.EventType = domain.AuditEventType(eventTypeStr)
		if fileIDStr != nil && *fileIDStr != "" {
			fileID, parseErr := uuid.Parse(*fileIDStr)
			if parseErr == nil {
				event.FileID = &fileID
			}
		}
		if jobIDStr != nil && *jobIDStr != "" {
			jobID, parseErr := uuid.Parse(*jobIDStr)
			if parseErr == nil {
				event.JobID = &jobID
			}
		}
		if detailsStr != "" {
			if err := json.Unmarshal([]byte(detailsStr), &event.Details); err != nil {
				return nil, fmt.Errorf("unmarshal audit details: %w", err)
			}
		}
		parsedCreatedAt, parseErr := parseTime(createdAt)
		if parseErr != nil {
			return nil, fmt.Errorf("parse audit created_at: %w", parseErr)
		}
		event.CreatedAt = parsedCreatedAt
		events = append(events, event)
	}

	return events, rows.Err()
}
