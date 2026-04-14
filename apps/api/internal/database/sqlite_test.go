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
