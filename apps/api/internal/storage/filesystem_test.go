package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type errReader struct {
	writes int
	err    error
}

func (r *errReader) Read(p []byte) (int, error) {
	if r.writes == 0 {
		r.writes++
		copy(p, []byte("partial"))
		return len("partial"), nil
	}
	return 0, r.err
}

func TestSaveArtifactRemovesDirectoryWhenWriteFails(t *testing.T) {
	store, err := NewFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("new filesystem: %v", err)
	}

	artifactID := "artifact-test"
	expectedErr := errors.New("boom")
	_, err = store.SaveArtifact(context.Background(), artifactID, "output.pdf", &errReader{err: expectedErr})
	if err == nil {
		t.Fatal("expected SaveArtifact to fail")
	}

	if _, statErr := os.Stat(filepath.Join(store.BasePath(), "artifacts", artifactID)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected artifact dir to be removed, got stat err %v", statErr)
	}
}

func TestSaveArtifactPersistsFileOnSuccess(t *testing.T) {
	store, err := NewFilesystem(t.TempDir())
	if err != nil {
		t.Fatalf("new filesystem: %v", err)
	}

	path, err := store.SaveArtifact(context.Background(), "artifact-ok", "output.txt", strings.NewReader("ok"))
	if err != nil {
		t.Fatalf("SaveArtifact: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("unexpected artifact contents: %q", string(data))
	}
}
