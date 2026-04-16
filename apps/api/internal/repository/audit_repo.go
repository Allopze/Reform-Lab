package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// AuditRepository persists audit events.
type AuditRepository interface {
	Create(ctx context.Context, e *domain.AuditEvent) error
	ListRecent(ctx context.Context, limit int, eventType *domain.AuditEventType) ([]domain.AuditEvent, error)
	ListForAdmin(ctx context.Context, filter AdminAuditFilter) (*AdminAuditPage, error)
}

// AdminAuditFilter defines pagination and filtering for admin audit listings.
type AdminAuditFilter struct {
	EventType string // exact event type; empty = all
	Prefix    string // optional event type prefix (e.g. admin_)
	Limit     int    // max page size: 200
	Offset    int
}

// AdminAuditPage is a paginated result for audit events.
type AdminAuditPage struct {
	Events []domain.AuditEvent `json:"events"`
	Total  int                 `json:"total"`
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

func (r *sqliteAuditRepo) ListForAdmin(ctx context.Context, filter AdminAuditFilter) (*AdminAuditPage, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	where := make([]string, 0, 2)
	args := make([]interface{}, 0, 4)

	if trimmedPrefix := strings.TrimSpace(filter.Prefix); trimmedPrefix != "" {
		where = append(where, "event_type LIKE ?")
		args = append(args, trimmedPrefix+"%")
	}
	if trimmedEventType := strings.TrimSpace(filter.EventType); trimmedEventType != "" {
		where = append(where, "event_type = ?")
		args = append(args, trimmedEventType)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	countQuery := `SELECT COUNT(*) FROM audit_events` + whereClause
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count admin audit events: %w", err)
	}

	listQuery := `SELECT id, event_type, file_id, job_id, details, created_at
		 FROM audit_events` + whereClause + `
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`

	listArgs := append(args, filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, fmt.Errorf("list admin audit events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.AuditEvent, 0, filter.Limit)
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &AdminAuditPage{Events: events, Total: total}, nil
}
