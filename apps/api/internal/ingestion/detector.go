package ingestion

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/gabriel-vasile/mimetype"
)

const maxDetectionBytes = 64 * 1024

// mimeToFamily maps known MIME types to format families.
var mimeToFamily = map[string]domain.FormatFamily{
	"application/pdf": domain.FamilyPDF,

	"image/jpeg":    domain.FamilyImage,
	"image/png":     domain.FamilyImage,
	"image/webp":    domain.FamilyImage,
	"image/gif":     domain.FamilyImage,
	"image/bmp":     domain.FamilyImage,
	"image/tiff":    domain.FamilyImage,
	"image/svg+xml": domain.FamilyImage,
	"image/heic":    domain.FamilyImage,
	"image/heif":    domain.FamilyImage,

	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   domain.FamilyDocument,
	"application/vnd.oasis.opendocument.text":                                   domain.FamilyDocument,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": domain.FamilyDocument,
	"application/vnd.oasis.opendocument.presentation":                           domain.FamilyDocument,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         domain.FamilyDocument,
	"application/vnd.oasis.opendocument.spreadsheet":                            domain.FamilyDocument,
	"text/html":          domain.FamilyDocument,
	"text/markdown":      domain.FamilyDocument,
	"text/plain":         domain.FamilyDocument,
	"text/csv":           domain.FamilyDocument,
	"application/rtf":    domain.FamilyDocument,
	"text/rtf":           domain.FamilyDocument,
	"application/msword": domain.FamilyDocument,

	"audio/mpeg":   domain.FamilyAudio,
	"audio/wav":    domain.FamilyAudio,
	"audio/ogg":    domain.FamilyAudio,
	"audio/opus":   domain.FamilyAudio,
	"audio/flac":   domain.FamilyAudio,
	"audio/aac":    domain.FamilyAudio,
	"audio/mp4":    domain.FamilyAudio,
	"audio/x-m4a":  domain.FamilyAudio,
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
	"image/svg+xml":   "svg",
	"image/heic":      "heic",
	"image/heif":      "heif",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   "docx",
	"application/vnd.oasis.opendocument.text":                                   "odt",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": "pptx",
	"application/vnd.oasis.opendocument.presentation":                           "odp",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         "xlsx",
	"application/vnd.oasis.opendocument.spreadsheet":                            "ods",
	"text/html":          "html",
	"text/markdown":      "md",
	"text/plain":         "txt",
	"text/csv":           "csv",
	"application/rtf":    "rtf",
	"text/rtf":           "rtf",
	"application/msword": "doc",
	"audio/mpeg":         "mp3",
	"audio/wav":          "wav",
	"audio/x-wav":        "wav",
	"audio/ogg":          "ogg",
	"audio/opus":         "opus",
	"audio/flac":         "flac",
	"audio/x-flac":       "flac",
	"audio/aac":          "aac",
	"audio/mp4":          "m4a",
	"audio/x-m4a":        "m4a",
	"video/mp4":          "mp4",
	"video/quicktime":    "mov",
	"video/webm":         "webm",
	"video/x-msvideo":    "avi",
}

// DetectFormat reads file content to determine its real type using magic bytes.
// It never trusts the file name or extension.
func DetectFormat(r io.ReadSeeker) (domain.DetectedFormat, error) {
	sample, err := readDetectionSample(r)
	if err != nil {
		return domain.DetectedFormat{}, err
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return domain.DetectedFormat{}, err
	}

	mtype, err := mimetype.DetectReader(bytes.NewReader(sample))
	if err != nil {
		return domain.DetectedFormat{}, err
	}

	mime := normalizeDetectedMIME(mtype.String())
	if looksLikeSVG(sample) {
		mime = "image/svg+xml"
	}
	if officeMime := detectOOXMLMime(r, sample); officeMime != "" {
		switch mime {
		case "application/zip", "application/octet-stream", "application/x-zip-compressed":
			mime = officeMime
		default:
			if _, known := mimeToFamily[mime]; !known {
				mime = officeMime
			}
		}
	}
	if mime == "audio/ogg" && looksLikeOpus(sample) {
		mime = "audio/opus"
	}
	if mime == "text/plain" {
		switch {
		case looksLikeMarkdown(sample):
			mime = "text/markdown"
		case looksLikeCSV(sample):
			mime = "text/csv"
		}
	}
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

func readDetectionSample(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxDetectionBytes))
}

var (
	markdownHeadingPattern  = regexp.MustCompile(`(?m)^#{1,6}\s+\S+`)
	markdownFencePattern    = regexp.MustCompile("(?m)^```")
	markdownBulletPattern   = regexp.MustCompile(`(?m)^\s*[-*+]\s+\S+`)
	markdownNumberedPattern = regexp.MustCompile(`(?m)^\s*\d+\.\s+\S+`)
	markdownLinkPattern     = regexp.MustCompile(`\[[^\]]+\]\([^\)]+\)`)
	markdownQuotePattern    = regexp.MustCompile(`(?m)^>\s+\S+`)
	markdownTablePattern    = regexp.MustCompile(`(?m)^\|.+\|$`)
	markdownRulePattern     = regexp.MustCompile(`(?m)^-{3,}$`)
)

func looksLikeMarkdown(sample []byte) bool {
	trimmed := strings.TrimSpace(string(sample))
	if trimmed == "" {
		return false
	}

	strongSignals := 0
	weakSignals := 0
	for _, signal := range []*regexp.Regexp{
		markdownHeadingPattern,
		markdownFencePattern,
		markdownBulletPattern,
		markdownNumberedPattern,
		markdownLinkPattern,
	} {
		if signal.MatchString(trimmed) {
			strongSignals++
		}
	}
	for _, signal := range []*regexp.Regexp{
		markdownQuotePattern,
		markdownTablePattern,
		markdownRulePattern,
	} {
		if signal.MatchString(trimmed) {
			weakSignals++
		}
	}

	if strongSignals >= 1 && weakSignals >= 1 {
		return true
	}
	return strongSignals >= 2
}

func looksLikeCSV(sample []byte) bool {
	text := strings.TrimSpace(string(sample))
	if text == "" {
		return false
	}

	var lines []string
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines = append(lines, trimmed)
		if len(lines) == 6 {
			break
		}
	}
	if len(lines) < 2 {
		return false
	}

	for _, separator := range []string{",", ";", "\t"} {
		counts := make([]int, 0, len(lines))
		valid := true
		for _, line := range lines {
			count := strings.Count(line, separator)
			if count == 0 {
				valid = false
				break
			}
			counts = append(counts, count)
		}
		if valid && len(counts) > 0 && counts[0] > 0 && slices.Equal(counts, repeatInt(counts[0], len(counts))) {
			return true
		}
	}

	return false
}

func looksLikeSVG(sample []byte) bool {
	text := strings.ToLower(strings.TrimSpace(string(sample)))
	if text == "" {
		return false
	}
	return strings.Contains(text, "<svg") && strings.Contains(text, "</svg>")
}

func looksLikeOpus(sample []byte) bool {
	return bytes.Contains(sample, []byte("OpusHead"))
}

func detectOOXMLMime(r io.ReadSeeker, sample []byte) string {
	if !bytes.HasPrefix(sample, []byte{'P', 'K', 0x03, 0x04}) {
		return ""
	}

	if readerAt, ok := r.(io.ReaderAt); ok {
		size, err := r.Seek(0, io.SeekEnd)
		if err == nil {
			if _, err := r.Seek(0, io.SeekStart); err == nil {
				if mime := detectOOXMLMimeFromZip(readerAt, size); mime != "" {
					return mime
				}
			}
		}
	}

	return detectOOXMLMimeFromZip(bytes.NewReader(sample), int64(len(sample)))
}

func detectOOXMLMimeFromZip(readerAt io.ReaderAt, size int64) string {
	zr, err := zip.NewReader(readerAt, size)
	if err != nil {
		return ""
	}

	for i, file := range zr.File {
		if i >= 128 {
			break
		}
		name := strings.ToLower(file.Name)
		switch {
		case strings.HasPrefix(name, "ppt/"):
			return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
		case strings.HasPrefix(name, "xl/"):
			return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		case strings.HasPrefix(name, "word/"):
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}
	}

	return ""
}

func repeatInt(value, count int) []int {
	values := make([]int, count)
	for i := range values {
		values[i] = value
	}
	return values
}

func normalizeDetectedMIME(value string) string {
	base, _, _ := strings.Cut(value, ";")
	return strings.TrimSpace(base)
}
