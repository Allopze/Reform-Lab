package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Filesystem implements Store using the local file system.
type Filesystem struct {
	basePath string
}

// NewFilesystem creates a filesystem-backed store rooted at basePath.
func NewFilesystem(basePath string) (*Filesystem, error) {
	dirs := []string{
		filepath.Join(basePath, "originals"),
		filepath.Join(basePath, "artifacts"),
		filepath.Join(basePath, "temp"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o750); err != nil {
			return nil, fmt.Errorf("create storage dir %s: %w", d, err)
		}
	}
	return &Filesystem{basePath: basePath}, nil
}

// BasePath returns the root path used by the filesystem-backed store.
func (fs *Filesystem) BasePath() string {
	return fs.basePath
}

func (fs *Filesystem) SaveOriginal(_ context.Context, fileID string, r io.Reader) (string, error) {
	dir := filepath.Join(fs.basePath, "originals", fileID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create original dir: %w", err)
	}
	p := filepath.Join(dir, "data")
	return p, writeFile(p, r)
}

func (fs *Filesystem) GetOriginal(_ context.Context, fileID string) (io.ReadCloser, error) {
	p := filepath.Join(fs.basePath, "originals", fileID, "data")
	return os.Open(p)
}

func (fs *Filesystem) OriginalPath(fileID string) string {
	return filepath.Join(fs.basePath, "originals", fileID, "data")
}

func (fs *Filesystem) SaveArtifact(_ context.Context, artifactID string, fileName string, r io.Reader) (string, error) {
	dir := filepath.Join(fs.basePath, "artifacts", artifactID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create artifact dir: %w", err)
	}
	p := filepath.Join(dir, fileName)
	return p, writeFile(p, r)
}

func (fs *Filesystem) GetArtifact(_ context.Context, artifactID string) (io.ReadCloser, error) {
	return fs.GetArtifactByName(artifactID, "")
}

// GetArtifactByName opens a specific file inside an artifact directory.
// If fileName is empty it falls back to the single entry in the directory.
func (fs *Filesystem) GetArtifactByName(artifactID, fileName string) (io.ReadCloser, error) {
	dir := filepath.Join(fs.basePath, "artifacts", artifactID)
	if fileName != "" {
		return os.Open(filepath.Join(dir, fileName))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read artifact dir: %w", err)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("artifact dir is empty")
	}
	if len(entries) > 1 {
		return nil, fmt.Errorf("artifact dir has multiple files; specify fileName")
	}
	return os.Open(filepath.Join(dir, entries[0].Name()))
}

func (fs *Filesystem) ArtifactPath(artifactID string) string {
	return filepath.Join(fs.basePath, "artifacts", artifactID)
}

func (fs *Filesystem) CreateTempDir(_ context.Context, jobID string) (string, error) {
	dir := filepath.Join(fs.basePath, "temp", jobID)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	return dir, nil
}

func (fs *Filesystem) CleanupTemp(_ context.Context, jobID string) error {
	dir := filepath.Join(fs.basePath, "temp", jobID)
	return os.RemoveAll(dir)
}

func writeFile(path string, r io.Reader) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file %s: %w", path, err)
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	return nil
}
