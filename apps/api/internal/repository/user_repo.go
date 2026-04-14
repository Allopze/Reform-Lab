package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

// UserRepository persists and retrieves User records.
type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	Count(ctx context.Context) (int, error)
	HasAdmin(ctx context.Context) (bool, error)
}

type sqliteUserRepo struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &sqliteUserRepo{db: db}
}

func (r *sqliteUserRepo) Create(ctx context.Context, u *domain.User) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO users (id, name, email, team, role, password_hash, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID.String(), u.Name, u.Email, u.Team, string(u.Role), u.PasswordHash, u.CreatedAt.Format(timeLayout),
	)
	if err != nil && isUniqueViolation(err) && isEmailUniqueViolation(err) {
		return domain.ErrEmailAlreadyExists
	}
	return err
}

func (r *sqliteUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var u domain.User
	var idStr, createdAt, role string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, team, role, password_hash, created_at FROM users WHERE email = ?`, email,
	).Scan(&idStr, &u.Name, &u.Email, &u.Team, &role, &u.PasswordHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	u.ID, _ = uuid.Parse(idStr)
	u.Role = domain.UserRole(role)
	u.CreatedAt, _ = parseTime(createdAt)
	return &u, nil
}

func (r *sqliteUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var u domain.User
	var idStr, createdAt, role string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, team, role, password_hash, created_at FROM users WHERE id = ?`, id.String(),
	).Scan(&idStr, &u.Name, &u.Email, &u.Team, &role, &u.PasswordHash, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	u.ID, _ = uuid.Parse(idStr)
	u.Role = domain.UserRole(role)
	u.CreatedAt, _ = parseTime(createdAt)
	return &u, nil
}

func (r *sqliteUserRepo) Count(ctx context.Context) (int, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *sqliteUserRepo) HasAdmin(ctx context.Context) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// isUniqueViolation checks if a SQLite error is a UNIQUE constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && !errors.Is(err, sql.ErrNoRows) &&
		strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func isEmailUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "users.email") || strings.Contains(msg, "idx_users_email")
}
