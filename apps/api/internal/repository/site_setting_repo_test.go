package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/database"
)

func TestSiteSettingRepoUpsertValuesPersistsBatch(t *testing.T) {
	t.Parallel()

	db, err := database.Open(filepath.Join(t.TempDir(), "reform-test.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := database.Migrate(db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	repo := NewSiteSettingRepository(db)
	ctx := context.Background()
	now := time.Now().UTC()
	values := map[string]string{
		"setting.one": "first",
		"setting.two": "second",
	}

	if err := repo.UpsertValues(ctx, values, now); err != nil {
		t.Fatalf("UpsertValues: %v", err)
	}

	for key, want := range values {
		got, ok, err := repo.GetValue(ctx, key)
		if err != nil {
			t.Fatalf("GetValue(%s): %v", key, err)
		}
		if !ok {
			t.Fatalf("expected setting %s to exist", key)
		}
		if got != want {
			t.Fatalf("setting %s: expected %q, got %q", key, want, got)
		}
	}
}
