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

const maxImagePixels = 40_000_000
const maxPDFPages = 500
const maxAudioDurationSec = 60 * 60
const maxVideoDurationSec = 30 * 60

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

	if meta.Width != nil && meta.Height != nil {
		if int64(*meta.Width)*int64(*meta.Height) > maxImagePixels {
			return domain.ErrLimitExceeded
		}
	} else if format.Family == domain.FamilyImage {
		// Dimensions could not be extracted (e.g. HEIF/SVG with no parseable header).
		// Enforce a conservative file size limit to prevent decompression bombs.
		const maxUnknownDimImageBytes int64 = 10 * 1024 * 1024 // 10 MB
		if size > maxUnknownDimImageBytes {
			return domain.ErrLimitExceeded
		}
	}

	if meta.Pages != nil && format.Family == domain.FamilyPDF {
		if *meta.Pages > maxPDFPages {
			return domain.ErrLimitExceeded
		}
	}

	if meta.DurationSec != nil {
		switch format.Family {
		case domain.FamilyAudio:
			if *meta.DurationSec > maxAudioDurationSec {
				return domain.ErrLimitExceeded
			}
		case domain.FamilyVideo:
			if *meta.DurationSec > maxVideoDurationSec {
				return domain.ErrLimitExceeded
			}
		}
	}

	if meta.IsProtected {
		return domain.ErrProtectedUnsupported
	}

	return nil
}
