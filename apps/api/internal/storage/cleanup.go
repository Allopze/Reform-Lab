package storage

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

// FileExpirer marks and deletes expired file records.
type FileExpirer interface {
	MarkExpiredByInternalName(ctx context.Context, internalName string) error
	DeleteExpiredBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

// CleanupService purges stale originals and temp directories from local storage.
type CleanupService struct {
	basePath    string
	logger      zerolog.Logger
	originalTTL time.Duration
	tempTTL     time.Duration
	files       FileExpirer // optional; nil skips DB cleanup
}

func NewCleanupService(basePath string, logger zerolog.Logger, originalTTL, tempTTL time.Duration) *CleanupService {
	return &CleanupService{
		basePath:    basePath,
		logger:      logger,
		originalTTL: originalTTL,
		tempTTL:     tempTTL,
	}
}

// WithFileExpirer attaches a FileExpirer so stale DB records are marked and purged.
func (s *CleanupService) WithFileExpirer(f FileExpirer) *CleanupService {
	s.files = f
	return s
}

func (s *CleanupService) Start(ctx context.Context, interval time.Duration) {
	s.runOnce()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce()
		}
	}
}

func (s *CleanupService) runOnce() {
	if s.originalTTL > 0 {
		s.cleanupExpiredDirs(filepath.Join(s.basePath, "originals"), s.originalTTL, "original")
	}
	if s.tempTTL > 0 {
		s.cleanupExpiredDirs(filepath.Join(s.basePath, "temp"), s.tempTTL, "temp")
	}
	// Purge file records that were marked expired more than 7 days ago.
	if s.files != nil {
		cutoff := time.Now().UTC().Add(-7 * 24 * time.Hour)
		n, err := s.files.DeleteExpiredBefore(context.Background(), cutoff)
		if err != nil {
			s.logger.Error().Err(err).Msg("delete expired file records failed")
		} else if n > 0 {
			s.logger.Info().Int64("count", n).Msg("expired file records purged")
		}
	}
}

func (s *CleanupService) cleanupExpiredDirs(root string, ttl time.Duration, kind string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if !os.IsNotExist(err) {
			s.logger.Error().Err(err).Str("root", root).Str("kind", kind).Msg("list cleanup root failed")
		}
		return
	}

	now := time.Now().UTC()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(root, entry.Name())
		modTime, err := latestModTime(dirPath)
		if err != nil {
			s.logger.Error().Err(err).Str("path", dirPath).Str("kind", kind).Msg("inspect cleanup candidate failed")
			continue
		}
		if now.Sub(modTime) < ttl {
			continue
		}
		if err := os.RemoveAll(dirPath); err != nil {
			s.logger.Error().Err(err).Str("path", dirPath).Str("kind", kind).Msg("remove expired storage dir failed")
			continue
		}
		s.logger.Info().Str("path", dirPath).Str("kind", kind).Msg("expired storage dir purged")

		// Mark the corresponding file record as expired when purging originals.
		if kind == "original" && s.files != nil {
			if markErr := s.files.MarkExpiredByInternalName(context.Background(), entry.Name()); markErr != nil {
				s.logger.Error().Err(markErr).Str("internal_name", entry.Name()).Msg("mark file record expired failed")
			}
		}
	}
}

func latestModTime(dirPath string) (time.Time, error) {
	info, err := os.Stat(dirPath)
	if err != nil {
		return time.Time{}, err
	}
	latest := info.ModTime()
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return latest, nil
	}
	for _, entry := range entries {
		entryInfo, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if entryInfo.ModTime().After(latest) {
			latest = entryInfo.ModTime()
		}
	}
	return latest, nil
}
