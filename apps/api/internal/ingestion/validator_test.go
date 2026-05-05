package ingestion

import (
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

func TestValidateFileRejectsPDFsWithTooManyPages(t *testing.T) {
	pages := maxPDFPages + 1

	err := ValidateFile(1024, domain.DetectedFormat{Family: domain.FamilyPDF}, domain.FileMetadata{
		Pages: &pages,
	})
	if err != domain.ErrLimitExceeded {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}
}

func TestValidateFileRejectsVideoThatRunsTooLong(t *testing.T) {
	duration := float64(maxVideoDurationSec + 1)

	err := ValidateFile(1024, domain.DetectedFormat{Family: domain.FamilyVideo}, domain.FileMetadata{
		DurationSec: &duration,
	})
	if err != domain.ErrLimitExceeded {
		t.Fatalf("expected ErrLimitExceeded, got %v", err)
	}
}

func TestValidateFileAllowsReasonableAudioDuration(t *testing.T) {
	duration := float64(maxAudioDurationSec - 30)

	err := ValidateFile(1024, domain.DetectedFormat{Family: domain.FamilyAudio}, domain.FileMetadata{
		DurationSec: &duration,
	})
	if err != nil {
		t.Fatalf("expected audio file to pass validation, got %v", err)
	}
}

func TestValidateFileAllowsUnknownDimensionImagesUpToConservativeLimit(t *testing.T) {
	err := ValidateFile(25*1024*1024, domain.DetectedFormat{Family: domain.FamilyImage}, domain.FileMetadata{})
	if err != nil {
		t.Fatalf("expected unknown-dimension image at conservative limit to pass, got %v", err)
	}
}

func TestValidateFileRejectsUnknownDimensionImagesWithSpecificError(t *testing.T) {
	err := ValidateFile(25*1024*1024+1, domain.DetectedFormat{Family: domain.FamilyImage}, domain.FileMetadata{})
	if err != domain.ErrImageDimensionsUnknown {
		t.Fatalf("expected ErrImageDimensionsUnknown, got %v", err)
	}
}
