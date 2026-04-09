package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/observability"
)

func TestCleanupServicePurgesExpiredOriginalsAndTemps(t *testing.T) {
	root := t.TempDir()
	originalDir := filepath.Join(root, "originals", "old-file")
	tempDir := filepath.Join(root, "temp", "old-job")
	if err := os.MkdirAll(originalDir, 0o755); err != nil {
		t.Fatalf("create original dir: %v", err)
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(originalDir, "data"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write original data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "work.tmp"), []byte("content"), 0o644); err != nil {
		t.Fatalf("write temp data: %v", err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(originalDir, oldTime, oldTime); err != nil {
		t.Fatalf("age original dir: %v", err)
	}
	if err := os.Chtimes(filepath.Join(originalDir, "data"), oldTime, oldTime); err != nil {
		t.Fatalf("age original file: %v", err)
	}
	if err := os.Chtimes(tempDir, oldTime, oldTime); err != nil {
		t.Fatalf("age temp dir: %v", err)
	}
	if err := os.Chtimes(filepath.Join(tempDir, "work.tmp"), oldTime, oldTime); err != nil {
		t.Fatalf("age temp file: %v", err)
	}

	service := NewCleanupService(root, observability.NewLogger("disabled"), 24*time.Hour, 6*time.Hour)
	service.runOnce()

	if _, err := os.Stat(originalDir); !os.IsNotExist(err) {
		t.Fatalf("expected original dir removed, stat err=%v", err)
	}
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Fatalf("expected temp dir removed, stat err=%v", err)
	}
}
