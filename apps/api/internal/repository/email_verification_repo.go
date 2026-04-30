package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// EmailVerificationRepository persists one-time email verification tokens.
type EmailVerificationRepository interface {
	DeleteForUser(ctx context.Context, userID uuid.UUID) error
	Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time, createdAt time.Time) error
	Consume(ctx context.Context, tokenHash string, usedAt time.Time) (uuid.UUID, error)
}

type sqliteEmailVerificationRepo struct {
	db *sql.DB
}

func NewEmailVerificationRepository(db *sql.DB) EmailVerificationRepository {
	return &sqliteEmailVerificationRepo{db: db}
}

func (r *sqliteEmailVerificationRepo) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = ?`, userID.String())
	return err
}

func (r *sqliteEmailVerificationRepo) Create(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time, createdAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, used_at, created_at)
		 VALUES (?, ?, ?, ?, NULL, ?)`,
		uuid.New().String(),
		userID.String(),
		tokenHash,
		expiresAt.UTC().Format(timeLayout),
		createdAt.UTC().Format(timeLayout),
	)
	return err
}

func (r *sqliteEmailVerificationRepo) Consume(ctx context.Context, tokenHash string, usedAt time.Time) (uuid.UUID, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var userIDStr string
	var expiresAtStr string
	var usedAtStr sql.NullString
	row := tx.QueryRowContext(ctx,
		`SELECT user_id, expires_at, used_at
		 FROM email_verification_tokens
		 WHERE token_hash = ?`,
		tokenHash,
	)
	if err := row.Scan(&userIDStr, &expiresAtStr, &usedAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.UUID{}, domain.ErrEmailVerificationTokenInvalid
		}
		return uuid.UUID{}, fmt.Errorf("scan email verification token: %w", err)
	}

	if usedAtStr.Valid && usedAtStr.String != "" {
		return uuid.UUID{}, domain.ErrEmailVerificationTokenInvalid
	}
	parsedExpires, err := parseTime(expiresAtStr)
	if err != nil {
		return uuid.UUID{}, domain.ErrEmailVerificationTokenInvalid
	}
	if usedAt.UTC().After(parsedExpires.UTC()) {
		return uuid.UUID{}, domain.ErrEmailVerificationTokenInvalid
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE email_verification_tokens
		 SET used_at = ?
		 WHERE token_hash = ? AND used_at IS NULL`,
		usedAt.UTC().Format(timeLayout),
		tokenHash,
	)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("mark email verification token used: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return uuid.UUID{}, domain.ErrEmailVerificationTokenInvalid
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.UUID{}, domain.ErrEmailVerificationTokenInvalid
	}

	if err := tx.Commit(); err != nil {
		return uuid.UUID{}, fmt.Errorf("commit consume email verification token: %w", err)
	}
	return userID, nil
}
