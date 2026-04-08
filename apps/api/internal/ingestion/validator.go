package ingestion

import (
	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

// maxSizeByFamily defines upload size limits per format family.
var maxSizeByFamily = map[domain.FormatFamily]int64{
	domain.FamilyPDF:      100 * 1024 * 1024, // 100 MB
	domain.FamilyImage:    100 * 1024 * 1024,
	domain.FamilyDocument: 100 * 1024 * 1024,
	domain.FamilyAudio:    250 * 1024 * 1024, // 250 MB
	domain.FamilyVideo:    500 * 1024 * 1024, // 500 MB
}

// ValidateFile checks that a file meets ingestion requirements.
// It returns a classified domain error on failure.
func ValidateFile(size int64, format domain.DetectedFormat, meta domain.FileMetadata) error {
	if size == 0 {
		return domain.ErrInvalidCorrupted
	}

	if format.Family == "" {
		return domain.ErrFormatUnsupported
	}

	maxSize, ok := maxSizeByFamily[format.Family]
	if !ok {
		return domain.ErrFormatUnsupported
	}

	if size > maxSize {
		return domain.ErrLimitExceeded
	}

	if meta.IsProtected {
		return domain.ErrProtectedUnsupported
	}

	return nil
}
