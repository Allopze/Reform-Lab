package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at the given path and runs migrations.
func Open(dbPath string) (*sql.DB, error) {
	if err := ensureWritableDatabasePath(dbPath); err != nil {
		return nil, err
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

func ensureWritableDatabasePath(dbPath string) error {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	file, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("prepare sqlite file %q: %w", dbPath, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close sqlite file %q: %w", dbPath, err)
	}

	return nil
}

// Migrate runs all SQL migration files against the database.
func Migrate(db *sql.DB, migrationsPath string) error {
	// Ensure tracking table exists.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		name TEXT PRIMARY KEY,
		applied_at DATETIME DEFAULT (datetime('now'))
	)`); err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	migrations, err := listMigrationFiles(migrationsPath)
	if err != nil {
		return err
	}
	for _, name := range migrations {
		// Skip already-applied migrations.
		var count int
		if err := db.QueryRow(`SELECT COUNT(*) FROM _migrations WHERE name = ?`, name).Scan(&count); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if count > 0 {
			continue
		}

		data, err := os.ReadFile(filepath.Join(migrationsPath, name))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := execMigrationStatements(db, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := db.Exec(`INSERT INTO _migrations (name) VALUES (?)`, name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}
	return nil
}

func listMigrationFiles(migrationsPath string) ([]string, error) {
	entries, err := os.ReadDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	migrations := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".sql" {
			continue
		}
		migrations = append(migrations, name)
	}

	if len(migrations) == 0 {
		return nil, fmt.Errorf("no migration files found in %q", migrationsPath)
	}

	sort.Strings(migrations)
	return migrations, nil
}

func execMigrationStatements(db *sql.DB, rawSQL string) error {
	statements := splitSQLStatements(rawSQL)
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

// splitSQLStatements splits raw SQL on semicolons that are NOT inside string literals.
func splitSQLStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	quote := byte(0)

	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if inString {
			current.WriteByte(ch)
			if ch == quote {
				// Check for escaped quote (doubled quote in SQL).
				if i+1 < len(sql) && sql[i+1] == quote {
					current.WriteByte(sql[i+1])
					i++
				} else {
					inString = false
				}
			}
		} else {
			if ch == '\'' || ch == '"' {
				inString = true
				quote = ch
				current.WriteByte(ch)
			} else if ch == ';' {
				statements = append(statements, current.String())
				current.Reset()
			} else {
				current.WriteByte(ch)
			}
		}
	}
	if s := current.String(); strings.TrimSpace(s) != "" {
		statements = append(statements, s)
	}
	return statements
}
