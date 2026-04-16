package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	ListAll(ctx context.Context) ([]domain.User, error)
	ListForAdmin(ctx context.Context, filter AdminUserFilter) (*AdminUserPage, error)
	UpdateRole(ctx context.Context, id uuid.UUID, role domain.UserRole) error
}

// AdminUserFilter defines filters for admin user listing.
type AdminUserFilter struct {
	Search string // free text match against name, email and team
	Role   string // empty = all
	Limit  int    // page size (max 100, default 50)
	Offset int
}

// AdminUserPage is a paginated result for admin users.
type AdminUserPage struct {
	Users []domain.User `json:"users"`
	Total int           `json:"total"`
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

func (r *sqliteUserRepo) ListAll(ctx context.Context) ([]domain.User, error) {
	page, err := r.ListForAdmin(ctx, AdminUserFilter{Limit: 10000, Offset: 0})
	if err != nil {
		return nil, err
	}
	return page.Users, nil
}

func (r *sqliteUserRepo) ListForAdmin(ctx context.Context, filter AdminUserFilter) (*AdminUserPage, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	where := "1=1"
	args := []interface{}{}

	if filter.Role != "" {
		where += " AND role = ?"
		args = append(args, filter.Role)
	}
	if filter.Search != "" {
		where += " AND (name LIKE ? OR email LIKE ? OR team LIKE ?)"
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like)
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM users WHERE %s`, where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count admin users: %w", err)
	}

	query := fmt.Sprintf(
		`SELECT id, name, email, team, role, created_at
		 FROM users
		 WHERE %s
		 ORDER BY created_at DESC
		 LIMIT ? OFFSET ?`, where,
	)
	pageArgs := append(args, filter.Limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, query, pageArgs...)
	if err != nil {
		return nil, fmt.Errorf("list admin users: %w", err)
	}
	defer rows.Close()

	users := make([]domain.User, 0, filter.Limit)
	for rows.Next() {
		var u domain.User
		var idStr, role, createdAt string
		if err := rows.Scan(&idStr, &u.Name, &u.Email, &u.Team, &role, &createdAt); err != nil {
			return nil, fmt.Errorf("scan admin user row: %w", err)
		}
		u.ID, _ = uuid.Parse(idStr)
		u.Role = domain.UserRole(role)
		u.CreatedAt, _ = parseTime(createdAt)
		users = append(users, u)
	}

	return &AdminUserPage{Users: users, Total: total}, nil
}

func (r *sqliteUserRepo) UpdateRole(ctx context.Context, id uuid.UUID, role domain.UserRole) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET role = ? WHERE id = ?`,
		string(role), id.String(),
	)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}
