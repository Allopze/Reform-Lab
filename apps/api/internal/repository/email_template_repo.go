package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

// EmailTemplateRepository manages email template persistence.
type EmailTemplateRepository interface {
	GetByKey(ctx context.Context, key string) (*domain.EmailTemplate, error)
	Upsert(ctx context.Context, tmpl *domain.EmailTemplate) error
	Delete(ctx context.Context, key string) error
	ListAll(ctx context.Context) ([]domain.EmailTemplate, error)
}

type sqliteEmailTemplateRepo struct {
	db *sql.DB
}

// NewEmailTemplateRepository creates a new SQLite-backed email template repository.
func NewEmailTemplateRepository(db *sql.DB) EmailTemplateRepository {
	return &sqliteEmailTemplateRepo{db: db}
}

func (r *sqliteEmailTemplateRepo) GetByKey(ctx context.Context, key string) (*domain.EmailTemplate, error) {
	var tmpl domain.EmailTemplate
	var updatedAt string
	err := r.db.QueryRowContext(ctx,
		`SELECT template_key, subject, body_html, updated_at
		 FROM email_templates
		 WHERE template_key = ?`, key,
	).Scan(&tmpl.Key, &tmpl.Subject, &tmpl.BodyHTML, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	tmpl.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &tmpl, nil
}

func (r *sqliteEmailTemplateRepo) Upsert(ctx context.Context, tmpl *domain.EmailTemplate) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO email_templates (template_key, subject, body_html, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(template_key) DO UPDATE SET
		   subject = excluded.subject,
		   body_html = excluded.body_html,
		   updated_at = excluded.updated_at`,
		tmpl.Key, tmpl.Subject, tmpl.BodyHTML, tmpl.UpdatedAt.UTC().Format(time.RFC3339),
	)
	return err
}

func (r *sqliteEmailTemplateRepo) ListAll(ctx context.Context) ([]domain.EmailTemplate, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT template_key, subject, body_html, updated_at
		 FROM email_templates
		 ORDER BY template_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []domain.EmailTemplate
	for rows.Next() {
		var tmpl domain.EmailTemplate
		var updatedAt string
		if err := rows.Scan(&tmpl.Key, &tmpl.Subject, &tmpl.BodyHTML, &updatedAt); err != nil {
			return nil, err
		}
		tmpl.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		templates = append(templates, tmpl)
	}
	return templates, rows.Err()
}

func (r *sqliteEmailTemplateRepo) Delete(ctx context.Context, key string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM email_templates WHERE template_key = ?`, key)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
