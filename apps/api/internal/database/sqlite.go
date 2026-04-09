package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at the given path and runs migrations.
func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// SQLite works best with a single writer.
	db.SetMaxOpenConns(1)

	return db, nil
}

// Migrate runs all SQL migration files against the database.
func Migrate(db *sql.DB, migrationsPath string) error {
	migrations := []string{"001_initial.sql", "002_users.sql", "003_owner_roles.sql", "004_site_settings.sql"}
	for _, name := range migrations {
		data, err := os.ReadFile(filepath.Join(migrationsPath, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := execMigrationStatements(db, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

func execMigrationStatements(db *sql.DB, rawSQL string) error {
	statements := strings.Split(rawSQL, ";")
	for _, statement := range statements {
		trimmed := strings.TrimSpace(statement)
		if trimmed == "" {
			continue
		}
		lines := strings.Split(trimmed, "\n")
		parts := make([]string, 0, len(lines))
		for _, line := range lines {
			clean := strings.TrimSpace(line)
			if strings.HasPrefix(clean, "--") || clean == "" {
				continue
			}
			parts = append(parts, line)
		}
		trimmed = strings.TrimSpace(strings.Join(parts, "\n"))
		if trimmed == "" {
			continue
		}
		if _, err := db.Exec(trimmed); err != nil {
			if strings.Contains(err.Error(), "duplicate column name") {
				continue
			}
			return err
		}
	}
	return nil
}
