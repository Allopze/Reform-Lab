package storage

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
)

// CleanupService purges stale originals and temp directories from local storage.
type CleanupService struct {
	basePath    string
	logger      zerolog.Logger
	originalTTL time.Duration
	tempTTL     time.Duration
}

func NewCleanupService(basePath string, logger zerolog.Logger, originalTTL, tempTTL time.Duration) *CleanupService {
	return &CleanupService{
		basePath:    basePath,
		logger:      logger,
		originalTTL: originalTTL,
		tempTTL:     tempTTL,
	}
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
