package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCreatesDatabaseFile(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "reform.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("stat db file: %v", err)
	}
}

func TestOpenReportsWritablePathError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permission test is not reliable when running as root")
	}

	root := t.TempDir()
	blockedDir := filepath.Join(root, "blocked")
	if err := os.Mkdir(blockedDir, 0o555); err != nil {
		t.Fatalf("mkdir blocked dir: %v", err)
	}

	dbPath := filepath.Join(blockedDir, "reform.db")
	_, err := Open(dbPath)
	if err == nil {
		t.Fatal("expected Open() to fail for non-writable directory")
	}

	if !strings.Contains(err.Error(), "prepare sqlite file") {
		t.Fatalf("expected writable-path error, got %v", err)
	}
}

func TestMigrateDiscoversAllSQLFiles(t *testing.T) {
	t.Parallel()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	migrationsDir := t.TempDir()
	files := map[string]string{
		"001_initial.sql":     `CREATE TABLE IF NOT EXISTS alpha (id INTEGER PRIMARY KEY);`,
		"002_second.sql":      `ALTER TABLE alpha ADD COLUMN name TEXT;`,
		"010_webhooks.sql":    `CREATE TABLE IF NOT EXISTS omega (id INTEGER PRIMARY KEY);`,
		"README.txt":          `ignored`,
		"009_placeholder.sql": `CREATE TABLE IF NOT EXISTS beta (id INTEGER PRIMARY KEY);`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(migrationsDir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write migration %s: %v", name, err)
		}
	}

	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	for _, table := range []string{"alpha", "beta", "omega"} {
		if _, err := db.Exec("SELECT 1 FROM " + table + " LIMIT 1"); err != nil {
			t.Fatalf("expected migration for table %s to be applied: %v", table, err)
		}
	}

	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM _migrations`).Scan(&applied); err != nil {
		t.Fatalf("count applied migrations: %v", err)
	}
	if applied != 4 {
		t.Fatalf("expected 4 applied SQL migrations, got %d", applied)
	}
}

func TestMigrateRealSchemaFromEmptyDatabase(t *testing.T) {
	t.Parallel()

	db, err := Open(filepath.Join(t.TempDir(), "reform.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("Migrate(real schema) error = %v", err)
	}
	if err := Migrate(db, migrationsDir); err != nil {
		t.Fatalf("second Migrate(real schema) error = %v", err)
	}

	for _, table := range []string{"users", "files", "jobs", "artifacts", "password_reset_tokens", "email_verification_tokens"} {
		if _, err := db.Exec("SELECT 1 FROM " + table + " LIMIT 1"); err != nil {
			t.Fatalf("expected real migration table %s: %v", table, err)
		}
	}

	columns := tableColumns(t, db, "users")
	if !columns["email_verified_at"] {
		t.Fatalf("expected users.email_verified_at column, got %v", columns)
	}

	var applied int
	if err := db.QueryRow(`SELECT COUNT(*) FROM _migrations`).Scan(&applied); err != nil {
		t.Fatalf("count real migrations: %v", err)
	}
	if applied == 0 {
		t.Fatal("expected real migrations to be recorded")
	}
}

func TestMigrateRealSchemaFromPasswordResetSnapshot(t *testing.T) {
	t.Parallel()

	db, err := Open(filepath.Join(t.TempDir(), "reform.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	realMigrationsDir := filepath.Join("..", "..", "migrations")
	snapshotDir := copyRealMigrationsThrough(t, realMigrationsDir, "015_password_reset_tokens.sql")
	if err := Migrate(db, snapshotDir); err != nil {
		t.Fatalf("Migrate(snapshot schema) error = %v", err)
	}

	if _, err := db.Exec(`SELECT email_verified_at FROM users LIMIT 1`); err == nil {
		t.Fatal("snapshot should not contain users.email_verified_at before migration 016")
	}

	if err := Migrate(db, realMigrationsDir); err != nil {
		t.Fatalf("Migrate(snapshot -> current schema) error = %v", err)
	}
	if err := Migrate(db, realMigrationsDir); err != nil {
		t.Fatalf("second Migrate(snapshot -> current schema) error = %v", err)
	}

	if _, err := db.Exec(`SELECT email_verified_at FROM users LIMIT 1`); err != nil {
		t.Fatalf("expected users.email_verified_at after migration 016: %v", err)
	}
	if _, err := db.Exec(`SELECT 1 FROM email_verification_tokens LIMIT 1`); err != nil {
		t.Fatalf("expected email_verification_tokens after migration 016: %v", err)
	}
}

func copyRealMigrationsThrough(t *testing.T, realMigrationsDir string, maxName string) string {
	t.Helper()

	names, err := listMigrationFiles(realMigrationsDir)
	if err != nil {
		t.Fatalf("list real migrations: %v", err)
	}
	targetDir := t.TempDir()
	for _, name := range names {
		if name > maxName {
			continue
		}
		data, err := os.ReadFile(filepath.Join(realMigrationsDir, name))
		if err != nil {
			t.Fatalf("read real migration %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(targetDir, name), data, 0o600); err != nil {
			t.Fatalf("write snapshot migration %s: %v", name, err)
		}
	}
	return targetDir
}

func tableColumns(t *testing.T, db *sql.DB, table string) map[string]bool {
	t.Helper()

	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table_info(%s): %v", table, err)
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info(%s): %v", table, err)
		}
		columns[name] = true
	}
	return columns
}
