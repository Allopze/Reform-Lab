package ingestion

import (
	"io"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/gabriel-vasile/mimetype"
)

// mimeToFamily maps known MIME types to format families.
var mimeToFamily = map[string]domain.FormatFamily{
	"application/pdf": domain.FamilyPDF,

	"image/jpeg": domain.FamilyImage,
	"image/png":  domain.FamilyImage,
	"image/webp": domain.FamilyImage,
	"image/gif":  domain.FamilyImage,
	"image/bmp":  domain.FamilyImage,
	"image/tiff": domain.FamilyImage,

	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": domain.FamilyDocument,
	"application/vnd.oasis.opendocument.text":                                 domain.FamilyDocument,
	"text/plain":         domain.FamilyDocument,
	"application/rtf":    domain.FamilyDocument,
	"text/rtf":           domain.FamilyDocument,
	"application/msword": domain.FamilyDocument,

	"audio/mpeg":   domain.FamilyAudio,
	"audio/wav":    domain.FamilyAudio,
	"audio/ogg":    domain.FamilyAudio,
	"audio/flac":   domain.FamilyAudio,
	"audio/aac":    domain.FamilyAudio,
	"audio/x-wav":  domain.FamilyAudio,
	"audio/x-flac": domain.FamilyAudio,

	"video/mp4":       domain.FamilyVideo,
	"video/quicktime": domain.FamilyVideo,
	"video/webm":      domain.FamilyVideo,
	"video/x-msvideo": domain.FamilyVideo,
}

// mimeToExtension maps MIME types to canonical file extensions.
var mimeToExtension = map[string]string{
	"application/pdf": "pdf",
	"image/jpeg":      "jpg",
	"image/png":       "png",
	"image/webp":      "webp",
	"image/gif":       "gif",
	"image/bmp":       "bmp",
	"image/tiff":      "tiff",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": "docx",
	"application/vnd.oasis.opendocument.text":                                 "odt",
	"text/plain":         "txt",
	"application/rtf":    "rtf",
	"text/rtf":           "rtf",
	"application/msword": "doc",
	"audio/mpeg":         "mp3",
	"audio/wav":          "wav",
	"audio/x-wav":        "wav",
	"audio/ogg":          "ogg",
	"audio/flac":         "flac",
	"audio/x-flac":       "flac",
	"audio/aac":          "aac",
	"video/mp4":          "mp4",
	"video/quicktime":    "mov",
	"video/webm":         "webm",
	"video/x-msvideo":    "avi",
}

// DetectFormat reads file content to determine its real type using magic bytes.
// It never trusts the file name or extension.
func DetectFormat(r io.Reader) (domain.DetectedFormat, error) {
	mtype, err := mimetype.DetectReader(r)
	if err != nil {
		return domain.DetectedFormat{}, err
	}

	mime := mtype.String()
	family, known := mimeToFamily[mime]
	if !known {
		return domain.DetectedFormat{
			MIMEType:   mime,
			Confidence: 1.0,
		}, domain.ErrFormatUnsupported
	}

	ext := mimeToExtension[mime]
	if ext == "" {
		ext = mtype.Extension()
	}

	return domain.DetectedFormat{
		MIMEType:   mime,
		Family:     family,
		Extension:  ext,
		Confidence: 1.0,
	}, nil
}
