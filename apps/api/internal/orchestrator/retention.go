package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/rs/zerolog"
)

type RetentionService struct {
	artifacts repository.ArtifactRepository
	jobs      repository.JobRepository
	logger    zerolog.Logger
}

func NewRetentionService(artifacts repository.ArtifactRepository, jobs repository.JobRepository, logger zerolog.Logger) *RetentionService {
	return &RetentionService{artifacts: artifacts, jobs: jobs, logger: logger}
}

func (s *RetentionService) Start(ctx context.Context, interval time.Duration) {
	s.runOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *RetentionService) runOnce(ctx context.Context) {
	expired, err := s.artifacts.ListExpiredBefore(ctx, time.Now().UTC(), 200)
	if err != nil {
		s.logger.Error().Err(err).Msg("list expired artifacts failed")
		return
	}

	for _, artifact := range expired {
		if err := os.RemoveAll(filepath.Dir(artifact.StoragePath)); err != nil {
			s.logger.Error().Err(err).Str("artifact_id", artifact.ID.String()).Msg("remove expired artifact from storage failed")
			continue
		}
		if err := s.jobs.ExpireArtifact(ctx, artifact.ID, time.Now().UTC()); err != nil {
			s.logger.Error().Err(err).Str("artifact_id", artifact.ID.String()).Msg("expire job for artifact failed")
			continue
		}
		if err := s.artifacts.DeleteByID(ctx, artifact.ID); err != nil {
			s.logger.Error().Err(err).Str("artifact_id", artifact.ID.String()).Msg("delete expired artifact record failed")
			continue
		}

		s.logger.Info().Str("artifact_id", artifact.ID.String()).Msg("expired artifact purged")
	}
}
