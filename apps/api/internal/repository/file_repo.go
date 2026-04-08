package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// FileRepository persists and retrieves OriginalFile records.
type FileRepository interface {
	Create(ctx context.Context, f *domain.OriginalFile) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.OriginalFile, error)
}

type sqliteFileRepo struct {
	db *sql.DB
}

// NewFileRepository creates a SQLite-backed FileRepository.
func NewFileRepository(db *sql.DB) FileRepository {
	return &sqliteFileRepo{db: db}
}

func (r *sqliteFileRepo) Create(ctx context.Context, f *domain.OriginalFile) error {
	meta, err := json.Marshal(f.Metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO files (id, user_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID.String(), nullableUUIDString(f.UserID), f.InternalName, f.OriginalName, f.Size,
		f.DetectedFormat.MIMEType, string(f.DetectedFormat.Family),
		f.DetectedFormat.Extension, string(meta), f.UploadedAt.Format(timeLayout),
	)
	return err
}

func (r *sqliteFileRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.OriginalFile, error) {
	var f domain.OriginalFile
	var family, idStr, metaStr, uploadedAt string
	var userIDStr *string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at
		 FROM files WHERE id = ?`, id.String(),
	).Scan(
		&idStr, &userIDStr, &f.InternalName, &f.OriginalName, &f.Size,
		&f.DetectedFormat.MIMEType, &family,
		&f.DetectedFormat.Extension, &metaStr, &uploadedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get file %s: %w", id, err)
	}

	f.ID = id
	if userIDStr != nil && *userIDStr != "" {
		uid, parseErr := uuid.Parse(*userIDStr)
		if parseErr == nil {
			f.UserID = &uid
		}
	}
	f.DetectedFormat.Family = domain.FormatFamily(family)
	if err := json.Unmarshal([]byte(metaStr), &f.Metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}
	t, err := parseTime(uploadedAt)
	if err != nil {
		return nil, fmt.Errorf("parse uploaded_at: %w", err)
	}
	f.UploadedAt = t
	return &f, nil
}
