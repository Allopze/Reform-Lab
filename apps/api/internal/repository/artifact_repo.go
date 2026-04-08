package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// ArtifactRepository persists and retrieves Artifact records.
type ArtifactRepository interface {
	Create(ctx context.Context, a *domain.Artifact) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error)
	ListExpiredBefore(ctx context.Context, before time.Time, limit int) ([]domain.Artifact, error)
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

type sqliteArtifactRepo struct {
	db *sql.DB
}

// NewArtifactRepository creates a SQLite-backed ArtifactRepository.
func NewArtifactRepository(db *sql.DB) ArtifactRepository {
	return &sqliteArtifactRepo{db: db}
}

func (r *sqliteArtifactRepo) Create(ctx context.Context, a *domain.Artifact) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO artifacts (id, user_id, job_id, file_id, file_name, mime_type, size, storage_path, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID.String(), nullableUUIDString(a.UserID), a.JobID.String(), a.FileID.String(), a.FileName,
		a.MIMEType, a.Size, a.StoragePath,
		a.CreatedAt.Format(timeLayout), a.ExpiresAt.Format(timeLayout),
	)
	return err
}

func (r *sqliteArtifactRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error) {
	var a domain.Artifact
	var idStr, jobIDStr, fileIDStr, createdAt, expiresAt string
	var userIDStr *string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, job_id, file_id, file_name, mime_type, size, storage_path, created_at, expires_at
		 FROM artifacts WHERE id = ?`, id.String(),
	).Scan(
		&idStr, &userIDStr, &jobIDStr, &fileIDStr, &a.FileName,
		&a.MIMEType, &a.Size, &a.StoragePath,
		&createdAt, &expiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get artifact %s: %w", id, err)
	}

	a.ID = id
	if userIDStr != nil && *userIDStr != "" {
		uid, parseErr := uuid.Parse(*userIDStr)
		if parseErr == nil {
			a.UserID = &uid
		}
	}
	if a.JobID, err = uuid.Parse(jobIDStr); err != nil {
		return nil, fmt.Errorf("parse artifact job_id: %w", err)
	}
	if a.FileID, err = uuid.Parse(fileIDStr); err != nil {
		return nil, fmt.Errorf("parse artifact file_id: %w", err)
	}
	if a.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse artifact created_at: %w", err)
	}
	if a.ExpiresAt, err = parseTime(expiresAt); err != nil {
		return nil, fmt.Errorf("parse artifact expires_at: %w", err)
	}
	return &a, nil
}

func (r *sqliteArtifactRepo) ListExpiredBefore(ctx context.Context, before time.Time, limit int) ([]domain.Artifact, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, job_id, file_id, file_name, mime_type, size, storage_path, created_at, expires_at
		 FROM artifacts
		 WHERE expires_at <= ?
		 ORDER BY expires_at ASC
		 LIMIT ?`,
		before.Format(timeLayout), limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	artifacts := make([]domain.Artifact, 0)
	for rows.Next() {
		var a domain.Artifact
		var idStr, jobIDStr, fileIDStr, createdAt, expiresAt string
		var userIDStr *string

		if err := rows.Scan(
			&idStr, &userIDStr, &jobIDStr, &fileIDStr, &a.FileName,
			&a.MIMEType, &a.Size, &a.StoragePath, &createdAt, &expiresAt,
		); err != nil {
			return nil, err
		}

		a.ID, _ = uuid.Parse(idStr)
		if userIDStr != nil && *userIDStr != "" {
			uid, parseErr := uuid.Parse(*userIDStr)
			if parseErr == nil {
				a.UserID = &uid
			}
		}
		a.JobID, _ = uuid.Parse(jobIDStr)
		a.FileID, _ = uuid.Parse(fileIDStr)
		a.CreatedAt, _ = parseTime(createdAt)
		a.ExpiresAt, _ = parseTime(expiresAt)
		artifacts = append(artifacts, a)
	}

	return artifacts, rows.Err()
}

func (r *sqliteArtifactRepo) DeleteByID(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM artifacts WHERE id = ?`, id.String())
	return err
}
