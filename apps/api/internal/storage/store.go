package storage

import (
	"context"
	"io"
)

// Store abstracts file persistence for originals, artifacts, and temporals.
// The implementation can be swapped (e.g. filesystem → S3) without changing callers.
type Store interface {
	// SaveOriginal persists an uploaded file under a UUID-based path.
	SaveOriginal(ctx context.Context, fileID string, r io.Reader) (path string, err error)

	// GetOriginal returns a reader for a previously saved original file.
	GetOriginal(ctx context.Context, fileID string) (io.ReadCloser, error)

	// OriginalPath returns the filesystem path to the original file (for workers).
	OriginalPath(fileID string) string

	// SaveArtifact persists a conversion result.
	SaveArtifact(ctx context.Context, artifactID string, fileName string, r io.Reader) (path string, err error)

	// GetArtifact returns a reader for a persisted artifact.
	GetArtifact(ctx context.Context, artifactID string) (io.ReadCloser, error)

	// GetArtifactByName opens a specific file inside an artifact directory.
	// If fileName is empty, falls back to GetArtifact behavior.
	GetArtifactByName(artifactID, fileName string) (io.ReadCloser, error)

	// ArtifactPath returns the filesystem path to the artifact.
	ArtifactPath(artifactID string) string

	// CreateTempDir creates a temp directory scoped to a job.
	CreateTempDir(ctx context.Context, jobID string) (path string, err error)

	// CleanupTemp removes the temp directory for a job.
	CleanupTemp(ctx context.Context, jobID string) error
}
