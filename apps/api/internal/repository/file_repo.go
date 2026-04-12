package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// FileRepository persists and retrieves OriginalFile records.
type FileRepository interface {
	Create(ctx context.Context, f *domain.OriginalFile) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.OriginalFile, error)
	MarkExpiredByInternalName(ctx context.Context, internalName string) error
	DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error)
	CumulativeBytesByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CumulativeBytesByGuestSession(ctx context.Context, sessionID uuid.UUID) (int64, error)
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
		`INSERT INTO files (id, user_id, guest_session_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ID.String(), nullableUUIDString(f.UserID), nullableUUIDString(f.GuestSessionID), f.InternalName, f.OriginalName, f.Size,
		f.DetectedFormat.MIMEType, string(f.DetectedFormat.Family),
		f.DetectedFormat.Extension, string(meta), f.UploadedAt.Format(timeLayout),
	)
	return err
}

func (r *sqliteFileRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.OriginalFile, error) {
	var f domain.OriginalFile
	var family, idStr, metaStr, uploadedAt string
	var userIDStr, guestSessionIDStr *string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, guest_session_id, internal_name, original_name, size, mime_type, format_family, detected_extension, metadata, uploaded_at
		 FROM files WHERE id = ?`, id.String(),
	).Scan(
		&idStr, &userIDStr, &guestSessionIDStr, &f.InternalName, &f.OriginalName, &f.Size,
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
	if guestSessionIDStr != nil && *guestSessionIDStr != "" {
		guestID, parseErr := uuid.Parse(*guestSessionIDStr)
		if parseErr == nil {
			f.GuestSessionID = &guestID
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

func (r *sqliteFileRepo) MarkExpiredByInternalName(ctx context.Context, internalName string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE files SET expired_at = ? WHERE internal_name = ? AND expired_at IS NULL`,
		time.Now().UTC().Format(timeLayout), internalName,
	)
	return err
}

func (r *sqliteFileRepo) DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM files WHERE expired_at IS NOT NULL AND expired_at < ?`,
		cutoff.Format(timeLayout),
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *sqliteFileRepo) CumulativeBytesByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size), 0) FROM files WHERE user_id = ? AND expired_at IS NULL`,
		userID.String(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("cumulative bytes by user: %w", err)
	}
	return total.Int64, nil
}

func (r *sqliteFileRepo) CumulativeBytesByGuestSession(ctx context.Context, sessionID uuid.UUID) (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(size), 0) FROM files WHERE guest_session_id = ? AND expired_at IS NULL`,
		sessionID.String(),
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("cumulative bytes by guest session: %w", err)
	}
	return total.Int64, nil
}
