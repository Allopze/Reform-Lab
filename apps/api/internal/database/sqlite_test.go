package database

import (
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
