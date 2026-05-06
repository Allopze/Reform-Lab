package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type SiteSettingRepository interface {
	GetValue(ctx context.Context, key string) (string, bool, error)
	UpsertValue(ctx context.Context, key, value string, updatedAt time.Time) error
	UpsertValues(ctx context.Context, values map[string]string, updatedAt time.Time) error
}

type sqliteSiteSettingRepo struct {
	db *sql.DB
}

func NewSiteSettingRepository(db *sql.DB) SiteSettingRepository {
	return &sqliteSiteSettingRepo{db: db}
}

func (r *sqliteSiteSettingRepo) GetValue(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := r.db.QueryRowContext(ctx,
		`SELECT setting_value
		 FROM site_settings
		 WHERE setting_key = ?`,
		key,
	).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

func (r *sqliteSiteSettingRepo) UpsertValue(ctx context.Context, key, value string, updatedAt time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO site_settings (setting_key, setting_value, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(setting_key) DO UPDATE SET
		   setting_value = excluded.setting_value,
		   updated_at = excluded.updated_at`,
		key,
		value,
		updatedAt.Format(timeLayout),
	)
	return err
}

func (r *sqliteSiteSettingRepo) UpsertValues(ctx context.Context, values map[string]string, updatedAt time.Time) error {
	if len(values) == 0 {
		return nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO site_settings (setting_key, setting_value, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(setting_key) DO UPDATE SET
		   setting_value = excluded.setting_value,
		   updated_at = excluded.updated_at`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	updated := updatedAt.Format(timeLayout)
	for key, value := range values {
		if _, err := stmt.ExecContext(ctx, key, value, updated); err != nil {
			return err
		}
	}

	return tx.Commit()
}
