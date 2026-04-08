package capabilities

import "github.com/allopze/reform-lab/apps/api/internal/domain"

const (
	mb100 = 100 * 1024 * 1024
	mb250 = 250 * 1024 * 1024
	mb500 = 500 * 1024 * 1024
)

// Catalog holds all declared capabilities for V1.
var Catalog = []domain.Capability{
	// ── PDF ──────────────────────────────────────────────
	{
		ID:            "pdf-to-docx",
		DisplayName:   "Convertir a Word",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpConvert,
		TargetFormat:  "docx",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Complex layouts may lose formatting",
		},
		Family: domain.FamilyPDF,
	},
	{
		ID:            "pdf-to-jpg",
		DisplayName:   "Convertir a JPG",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpConvert,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
		Engine:          "poppler",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Multi-page PDFs produce a ZIP of images",
		},
		Family: domain.FamilyPDF,
	},
	{
		ID:            "pdf-to-png",
		DisplayName:   "Convertir a PNG",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpConvert,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
		Engine:          "poppler",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Multi-page PDFs produce a ZIP of images",
		},
		Family: domain.FamilyPDF,
	},
	{
		ID:            "pdf-to-txt",
		DisplayName:   "Extraer texto",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpExtract,
		TargetFormat:  "txt",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 30,
			MaxRetries:     1,
		},
		Engine:          "poppler",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Image-only PDFs produce empty or minimal text",
		},
		Family: domain.FamilyPDF,
	},

	// ── Images ──────────────────────────────────────────
	{
		ID:          "image-to-png",
		DisplayName: "Convertir a PNG",
		SourceFormats: []string{
			"image/jpeg", "image/webp", "image/gif",
			"image/bmp", "image/tiff",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 30,
			MaxRetries:     1,
		},
		Engine:          "go-image",
		ExpectedQuality: "lossless",
		Family:          domain.FamilyImage,
	},
	{
		ID:          "image-to-jpg",
		DisplayName: "Convertir a JPG",
		SourceFormats: []string{
			"image/png", "image/webp", "image/gif",
			"image/bmp", "image/tiff",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 30,
			MaxRetries:     1,
		},
		Engine:          "go-image",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Lossy compression; transparency is flattened to white",
		},
		Family: domain.FamilyImage,
	},
	// image-to-webp removed: Go stdlib doesn't support WebP encoding natively.
	// Re-add when cwebp binary or cgo encoder is available.
	{
		ID:          "image-to-pdf",
		DisplayName: "Convertir a PDF",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 30,
			MaxRetries:     1,
		},
		Engine:          "go-image",
		ExpectedQuality: "high",
		Family:          domain.FamilyImage,
	},

	// ── Documents ───────────────────────────────────────
	{
		ID:          "doc-to-pdf",
		DisplayName: "Convertir a PDF",
		SourceFormats: []string{
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.oasis.opendocument.text",
			"application/rtf",
			"text/rtf",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Complex formatting may shift slightly",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:          "doc-to-txt",
		DisplayName: "Extraer texto",
		SourceFormats: []string{
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.oasis.opendocument.text",
			"application/rtf",
			"text/rtf",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "txt",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		Family:          domain.FamilyDocument,
	},
	{
		ID:          "doc-to-docx",
		DisplayName: "Convertir a Word",
		SourceFormats: []string{
			"application/vnd.oasis.opendocument.text",
			"application/rtf",
			"text/rtf",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "docx",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		Family:          domain.FamilyDocument,
	},

	// ── Audio ───────────────────────────────────────────
	{
		ID:          "audio-to-mp3",
		DisplayName: "Convertir a MP3",
		SourceFormats: []string{
			"audio/wav", "audio/x-wav", "audio/ogg",
			"audio/flac", "audio/x-flac", "audio/aac",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "mp3",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb250},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		Family:          domain.FamilyAudio,
	},
	{
		ID:          "audio-to-wav",
		DisplayName: "Convertir a WAV",
		SourceFormats: []string{
			"audio/mpeg", "audio/ogg",
			"audio/flac", "audio/x-flac", "audio/aac",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "wav",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb250},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "lossless",
		Family:          domain.FamilyAudio,
	},
	{
		ID:          "audio-to-ogg",
		DisplayName: "Convertir a OGG",
		SourceFormats: []string{
			"audio/mpeg", "audio/wav", "audio/x-wav",
			"audio/flac", "audio/x-flac", "audio/aac",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "ogg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb250},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		Family:          domain.FamilyAudio,
	},

	// ── Video ───────────────────────────────────────────
	{
		ID:          "video-to-mp4",
		DisplayName: "Convertir a MP4",
		SourceFormats: []string{
			"video/quicktime", "video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "mp4",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 600,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		Family:          domain.FamilyVideo,
	},
	{
		ID:          "video-to-webm",
		DisplayName: "Convertir a WebM",
		SourceFormats: []string{
			"video/mp4", "video/quicktime", "video/x-msvideo",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "webm",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 600,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		Family:          domain.FamilyVideo,
	},
	{
		ID:          "video-to-gif",
		DisplayName: "Convertir a GIF",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "gif",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 300,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "medium",
		KnownLimitations: []string{
			"Limited to first 30 seconds, 480px wide, 10 fps",
		},
		Family: domain.FamilyVideo,
	},
}

// ByID returns a capability by its ID, or nil if not found.
func ByID(id string) *domain.Capability {
	for i := range Catalog {
		if Catalog[i].ID == id {
			return &Catalog[i]
		}
	}
	return nil
}
