package capabilities

import "github.com/allopze/reform-lab/apps/api/internal/domain"

const (
	mb100 = 100 * 1024 * 1024
	mb250 = 250 * 1024 * 1024
	mb500 = 500 * 1024 * 1024
)

var capabilityPresentationOrders = map[string]int{
	"pdf-to-docx":            100,
	"pdf-to-jpg":             110,
	"pdf-to-png":             120,
	"pdf-to-txt":             130,
	"pdf-compress":           140,
	"pdf-to-html-preview":    150,
	"pdf-ocr-to-txt":         160,
	"pdf-ocr-searchable-pdf": 170,
	"pdf-ocr-to-json":        180,

	"image-to-jpg":        200,
	"image-to-png":        210,
	"image-to-webp":       220,
	"image-to-pdf":        230,
	"image-to-avif":       240,
	"image-compress-jpg":  250,
	"image-compress-png":  260,
	"image-web-jpg-1600":  270,
	"image-web-webp-1600": 280,
	"image-web-avif-1600": 290,
	"image-web-jpg-640":   300,
	"image-web-webp-640":  310,
	"image-web-avif-640":  320,
	"image-thumbnail-jpg": 330,
	"image-thumbnail-png": 340,
	"image-ocr-to-txt":    350,
	"image-ocr-to-json":   360,

	"image-heic-to-jpg":  400,
	"image-heic-to-png":  410,
	"image-heic-to-webp": 420,

	"image-svg-to-png":  500,
	"image-svg-to-webp": 510,
	"image-svg-to-pdf":  520,

	"doc-to-pdf":       600,
	"txt-to-pdf":       600,
	"html-to-pdf":      600,
	"doc-to-docx":      610,
	"doc-to-txt":       620,
	"doc-to-html":      630,
	"docx-to-markdown": 640,
	"markdown-to-html": 650,
	"markdown-to-pdf":  660,
	"markdown-to-docx": 670,
	"html-to-txt":      680,

	"presentation-to-pdf": 700,
	"presentation-to-jpg": 710,
	"presentation-to-png": 720,

	"spreadsheet-to-pdf":  800,
	"spreadsheet-to-xlsx": 810,
	"spreadsheet-to-csv":  820,
	"spreadsheet-to-html": 830,

	"audio-to-mp3":       900,
	"audio-to-m4a":       910,
	"audio-to-wav":       920,
	"audio-to-flac":      930,
	"audio-to-aac":       940,
	"audio-to-ogg":       950,
	"audio-to-opus":      960,
	"audio-waveform-png": 970,

	"video-to-mp4":        1000,
	"video-to-webm":       1010,
	"video-to-gif":        1020,
	"video-to-mp3":        1030,
	"video-to-m4a":        1040,
	"video-to-wav":        1050,
	"video-to-flac":       1060,
	"video-to-aac":        1070,
	"video-to-opus":       1080,
	"video-preview-mp4":   1090,
	"video-preview-webm":  1100,
	"video-to-thumbnails": 1110,
	"video-contact-sheet": 1120,
	"video-waveform-png":  1130,
}

func withPresentationOrders(capabilities []domain.Capability) []domain.Capability {
	for i := range capabilities {
		order, ok := capabilityPresentationOrders[capabilities[i].ID]
		if !ok {
			panic("missing presentation order for capability " + capabilities[i].ID)
		}
		capabilities[i].PresentationOrder = order
	}

	return capabilities
}

// Catalog holds all declared capabilities for V1.
var Catalog = withPresentationOrders([]domain.Capability{
	// ── PDF ──────────────────────────────────────────────
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
		ID:            "pdf-to-docx",
		DisplayName:   "Convertir a Word",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpConvert,
		TargetFormat:  "docx",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "pdf2docx",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Complex page layouts and scanned PDFs can lose fidelity or require OCR instead",
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
	{
		ID:            "pdf-compress",
		DisplayName:   "Comprimir PDF",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpCompress,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "ghostscript",
		ExpectedQuality: "medium",
		KnownLimitations: []string{
			"Compression can reduce image quality in scanned PDFs",
		},
		Family: domain.FamilyPDF,
	},
	{
		ID:            "pdf-to-html-preview",
		DisplayName:   "Generar preview HTML",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpPreview,
		TargetFormat:  "html",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 90,
			MaxRetries:     1,
		},
		Engine:          "poppler-html",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Preview output prioritizes readable HTML over exact visual fidelity",
		},
		Family: domain.FamilyPDF,
	},
	{
		ID:            "pdf-ocr-to-txt",
		DisplayName:   "OCR a TXT",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpExtract,
		TargetFormat:  "txt",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 240,
			MaxRetries:     1,
		},
		Engine:          "ocr-pdf",
		ExpectedQuality: "medium",
		KnownLimitations: []string{
			"OCR improves scanned PDFs but accuracy depends on scan quality, language, and page contrast",
		},
		Family: domain.FamilyPDF,
	},
	{
		ID:            "pdf-ocr-to-json",
		DisplayName:   "OCR a JSON",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpExtract,
		TargetFormat:  "json",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 240,
			MaxRetries:     1,
		},
		Engine:          "ocr-pdf",
		ExpectedQuality: "medium",
		KnownLimitations: []string{
			"Structured JSON is derived from OCR TSV blocks and lines, not from semantic PDF tags",
		},
		Family: domain.FamilyPDF,
	},
	{
		ID:            "pdf-ocr-searchable-pdf",
		DisplayName:   "OCR a PDF searchable",
		SourceFormats: []string{"application/pdf"},
		OperationType: domain.OpOptimize,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 300,
			MaxRetries:     1,
		},
		Engine:          "ocr-pdf",
		ExpectedQuality: "medium",
		KnownLimitations: []string{
			"Output rebuilds a searchable layer from OCR and may enlarge files compared with the original scan",
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
	{
		ID:          "image-to-webp",
		DisplayName: "Convertir a WebP",
		SourceFormats: []string{
			"image/jpeg", "image/png",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "webp",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Animated image sources are flattened to a single frame when unsupported",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-to-avif",
		DisplayName: "Convertir a AVIF",
		SourceFormats: []string{
			"image/jpeg", "image/png",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "avif",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Encoding uses a still-image AVIF path optimized for compact previews",
		},
		Family: domain.FamilyImage,
	},
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
	{
		ID:            "image-compress-jpg",
		DisplayName:   "Comprimir JPG",
		SourceFormats: []string{"image/jpeg"},
		OperationType: domain.OpCompress,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 30,
			MaxRetries:     1,
		},
		Engine:          "go-image",
		ExpectedQuality: "medium",
		KnownLimitations: []string{
			"Compression is lossy to reduce file size",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:            "image-compress-png",
		DisplayName:   "Comprimir PNG",
		SourceFormats: []string{"image/png"},
		OperationType: domain.OpCompress,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 30,
			MaxRetries:     1,
		},
		Engine:          "go-image",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"PNG compression preserves pixels but gains vary by image content",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:            "image-thumbnail-jpg",
		DisplayName:   "Crear thumbnail JPG",
		SourceFormats: []string{"image/jpeg"},
		OperationType: domain.OpPreview,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 20,
			MaxRetries:     1,
		},
		Engine:          "go-image",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Thumbnails are limited to 320px on the longest edge",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:            "image-thumbnail-png",
		DisplayName:   "Crear thumbnail PNG",
		SourceFormats: []string{"image/png"},
		OperationType: domain.OpPreview,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 20,
			MaxRetries:     1,
		},
		Engine:          "go-image",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Thumbnails are limited to 320px on the longest edge",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-ocr-to-txt",
		DisplayName: "OCR a TXT",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "txt",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "tesseract",
		ExpectedQuality: "medium",
		KnownLimitations: []string{
			"OCR accuracy depends on text size, contrast, and image cleanliness",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-ocr-to-json",
		DisplayName: "OCR a JSON",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "json",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "tesseract",
		ExpectedQuality: "medium",
		KnownLimitations: []string{
			"JSON reflects OCR blocks, lines, and words inferred from TSV coordinates rather than semantic regions",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-web-jpg-640",
		DisplayName: "Optimizar web JPG (640px)",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpOptimize,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Animated image sources are flattened to the first frame and the longest edge is capped at 640px",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-web-webp-640",
		DisplayName: "Optimizar web WebP (640px)",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpOptimize,
		TargetFormat:  "webp",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Animated image sources are flattened to the first frame and the longest edge is capped at 640px",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-web-avif-640",
		DisplayName: "Optimizar web AVIF (640px)",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpOptimize,
		TargetFormat:  "avif",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Animated image sources are flattened to the first frame and the longest edge is capped at 640px",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-web-jpg-1600",
		DisplayName: "Optimizar web JPG (1600px)",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpOptimize,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Animated image sources are flattened to the first frame and the longest edge is capped at 1600px",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-web-webp-1600",
		DisplayName: "Optimizar web WebP (1600px)",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpOptimize,
		TargetFormat:  "webp",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Animated image sources are flattened to the first frame and the longest edge is capped at 1600px",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-web-avif-1600",
		DisplayName: "Optimizar web AVIF (1600px)",
		SourceFormats: []string{
			"image/jpeg", "image/png", "image/webp",
			"image/gif", "image/bmp", "image/tiff",
		},
		OperationType: domain.OpOptimize,
		TargetFormat:  "avif",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Animated image sources are flattened to the first frame and the longest edge is capped at 1600px",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-heic-to-jpg",
		DisplayName: "Convertir HEIC/HEIF a JPG",
		SourceFormats: []string{
			"image/heic", "image/heif",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "libheif",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Conversion decodes the primary still image and sequence-specific metadata is not preserved",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-heic-to-png",
		DisplayName: "Convertir HEIC/HEIF a PNG",
		SourceFormats: []string{
			"image/heic", "image/heif",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "libheif",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Conversion decodes the primary still image and sequence-specific metadata is not preserved",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:          "image-heic-to-webp",
		DisplayName: "Convertir HEIC/HEIF a WebP",
		SourceFormats: []string{
			"image/heic", "image/heif",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "webp",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
		Engine:          "libheif",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Conversion decodes the primary still image before encoding to WebP",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:            "image-svg-to-png",
		DisplayName:   "Convertir SVG a PNG",
		SourceFormats: []string{"image/svg+xml"},
		OperationType: domain.OpConvert,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Vector output is rasterized into a single bitmap during export",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:            "image-svg-to-webp",
		DisplayName:   "Convertir SVG a WebP",
		SourceFormats: []string{"image/svg+xml"},
		OperationType: domain.OpConvert,
		TargetFormat:  "webp",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Vector output is rasterized into a single bitmap during export",
		},
		Family: domain.FamilyImage,
	},
	{
		ID:            "image-svg-to-pdf",
		DisplayName:   "Convertir SVG a PDF",
		SourceFormats: []string{"image/svg+xml"},
		OperationType: domain.OpConvert,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
		Engine:          "librsvg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Font availability and unsupported SVG features still depend on librsvg support in the runtime",
		},
		Family: domain.FamilyImage,
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
	{
		ID:            "doc-to-html",
		DisplayName:   "Convertir a HTML simple",
		SourceFormats: []string{"application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		OperationType: domain.OpConvert,
		TargetFormat:  "html",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 90,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Output prioritizes simple HTML over exact layout fidelity",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:            "txt-to-pdf",
		DisplayName:   "Convertir TXT a PDF",
		SourceFormats: []string{"text/plain"},
		OperationType: domain.OpConvert,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 45,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Plain text layout is intentionally simple",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:            "html-to-pdf",
		DisplayName:   "Convertir HTML a PDF",
		SourceFormats: []string{"text/html"},
		OperationType: domain.OpConvert,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 90,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Advanced CSS and scripted content may render differently in the PDF output",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:            "html-to-txt",
		DisplayName:   "Extraer texto limpio",
		SourceFormats: []string{"text/html"},
		OperationType: domain.OpExtract,
		TargetFormat:  "txt",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 15,
			MaxRetries:     1,
		},
		Engine:          "go-html",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Scripted content and advanced layout metadata are discarded while preserving readable text blocks",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:            "markdown-to-html",
		DisplayName:   "Convertir Markdown a HTML",
		SourceFormats: []string{"text/markdown"},
		OperationType: domain.OpConvert,
		TargetFormat:  "html",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 15,
			MaxRetries:     1,
		},
		Engine:          "goldmark",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Only Markdown files detected confidently from content are eligible",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:            "docx-to-markdown",
		DisplayName:   "Convertir DOCX a Markdown",
		SourceFormats: []string{"application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		OperationType: domain.OpConvert,
		TargetFormat:  "md",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 90,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Complex layout elements are simplified to preserve headings, lists, links, and basic emphasis",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:            "markdown-to-pdf",
		DisplayName:   "Convertir Markdown a PDF",
		SourceFormats: []string{"text/markdown"},
		OperationType: domain.OpConvert,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 60,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Rendered PDF follows a simple HTML export path",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:            "markdown-to-docx",
		DisplayName:   "Convertir Markdown a DOCX",
		SourceFormats: []string{"text/markdown"},
		OperationType: domain.OpConvert,
		TargetFormat:  "docx",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 90,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Rendered DOCX follows the same simplified HTML path as the Markdown PDF export",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:          "presentation-to-pdf",
		DisplayName: "Convertir presentación a PDF",
		SourceFormats: []string{
			"application/vnd.openxmlformats-officedocument.presentationml.presentation",
			"application/vnd.oasis.opendocument.presentation",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Complex transitions, embedded video, and unsupported fonts may be flattened during export",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:          "presentation-to-jpg",
		DisplayName: "Convertir presentación a JPG",
		SourceFormats: []string{
			"application/vnd.openxmlformats-officedocument.presentationml.presentation",
			"application/vnd.oasis.opendocument.presentation",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 240,
			MaxRetries:     1,
		},
		Engine:          "libreoffice-poppler",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Multi-slide presentations return one JPG per slide and may be bundled as ZIP",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:          "presentation-to-png",
		DisplayName: "Convertir presentación a PNG",
		SourceFormats: []string{
			"application/vnd.openxmlformats-officedocument.presentationml.presentation",
			"application/vnd.oasis.opendocument.presentation",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 240,
			MaxRetries:     1,
		},
		Engine:          "libreoffice-poppler",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Multi-slide presentations return one PNG per slide and may be bundled as ZIP",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:          "spreadsheet-to-pdf",
		DisplayName: "Convertir hoja de cálculo a PDF",
		SourceFormats: []string{
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/vnd.oasis.opendocument.spreadsheet",
			"text/csv",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "pdf",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"PDF export follows sheet print ranges and wide tables may be scaled across pages",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:          "spreadsheet-to-csv",
		DisplayName: "Exportar hoja de cálculo a CSV",
		SourceFormats: []string{
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/vnd.oasis.opendocument.spreadsheet",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "csv",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"CSV export preserves the active sheet only and flattens formulas to visible cell values",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:          "spreadsheet-to-xlsx",
		DisplayName: "Exportar hoja de cálculo a XLSX",
		SourceFormats: []string{
			"application/vnd.oasis.opendocument.spreadsheet",
			"text/csv",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "xlsx",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"CSV inputs become a single-sheet workbook and type inference follows LibreOffice import rules",
		},
		Family: domain.FamilyDocument,
	},
	{
		ID:          "spreadsheet-to-html",
		DisplayName: "Exportar hoja de cálculo a HTML",
		SourceFormats: []string{
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/vnd.oasis.opendocument.spreadsheet",
			"text/csv",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "html",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb100},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "libreoffice",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"HTML export prioritizes readable tables and can emit companion assets for richer formatting",
		},
		Family: domain.FamilyDocument,
	},

	// ── Audio ───────────────────────────────────────────
	{
		ID:          "audio-to-mp3",
		DisplayName: "Convertir a MP3",
		SourceFormats: []string{
			"audio/wav", "audio/x-wav", "audio/ogg",
			"audio/opus", "audio/flac", "audio/x-flac", "audio/aac",
			"audio/mp4", "audio/x-m4a",
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
			"audio/opus", "audio/flac", "audio/x-flac", "audio/aac",
			"audio/mp4", "audio/x-m4a",
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
			"audio/opus", "audio/flac", "audio/x-flac", "audio/aac",
			"audio/mp4", "audio/x-m4a",
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
	{
		ID:          "audio-to-aac",
		DisplayName: "Convertir a AAC",
		SourceFormats: []string{
			"audio/mpeg", "audio/wav", "audio/x-wav",
			"audio/ogg", "audio/opus", "audio/flac", "audio/x-flac",
			"audio/mp4", "audio/x-m4a",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "aac",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb250},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"AAC output is packaged as a lightweight ADTS stream",
		},
		Family: domain.FamilyAudio,
	},
	{
		ID:          "audio-to-m4a",
		DisplayName: "Convertir a M4A",
		SourceFormats: []string{
			"audio/mpeg", "audio/wav", "audio/x-wav",
			"audio/ogg", "audio/opus", "audio/flac", "audio/x-flac",
			"audio/aac", "audio/mp4", "audio/x-m4a",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "m4a",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb250},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"M4A output uses AAC audio inside an MP4 audio-only container",
		},
		Family: domain.FamilyAudio,
	},
	{
		ID:          "audio-to-flac",
		DisplayName: "Convertir a FLAC",
		SourceFormats: []string{
			"audio/mpeg", "audio/wav", "audio/x-wav",
			"audio/ogg", "audio/opus", "audio/aac",
			"audio/mp4", "audio/x-m4a",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "flac",
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
		ID:          "audio-to-opus",
		DisplayName: "Convertir a Opus",
		SourceFormats: []string{
			"audio/mpeg", "audio/wav", "audio/x-wav",
			"audio/ogg", "audio/opus", "audio/flac", "audio/x-flac",
			"audio/aac", "audio/mp4", "audio/x-m4a",
		},
		OperationType: domain.OpConvert,
		TargetFormat:  "opus",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb250},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Opus output is packaged in an Ogg-compatible stream optimized for speech and web playback",
		},
		Family: domain.FamilyAudio,
	},
	{
		ID:          "audio-waveform-png",
		DisplayName: "Generar waveform PNG",
		SourceFormats: []string{
			"audio/mpeg", "audio/wav", "audio/x-wav",
			"audio/ogg", "audio/opus", "audio/flac", "audio/x-flac", "audio/aac",
			"audio/mp4", "audio/x-m4a",
		},
		OperationType: domain.OpPreview,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb250},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 90,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Waveform preview summarizes the primary audio stream into a static PNG and omits metadata",
		},
		Family: domain.FamilyAudio,
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
	{
		ID:          "video-to-mp3",
		DisplayName: "Extraer audio MP3",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "mp3",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 600,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Only the primary audio stream is extracted",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-to-wav",
		DisplayName: "Extraer audio WAV",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "wav",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 600,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "lossless",
		KnownLimitations: []string{
			"Only the primary audio stream is extracted",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-to-aac",
		DisplayName: "Extraer audio AAC",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "aac",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 600,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Only the primary audio stream is extracted and output is packaged as ADTS AAC",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-to-m4a",
		DisplayName: "Extraer audio M4A",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "m4a",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 600,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Only the primary audio stream is extracted and repackaged as AAC in an M4A container",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-to-flac",
		DisplayName: "Extraer audio FLAC",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "flac",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 600,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "lossless",
		KnownLimitations: []string{
			"Only the primary audio stream is extracted",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-to-opus",
		DisplayName: "Extraer audio Opus",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpExtract,
		TargetFormat:  "opus",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 600,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "high",
		KnownLimitations: []string{
			"Only the primary audio stream is extracted and encoded as Opus",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-to-thumbnails",
		DisplayName: "Generar thumbnails",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpPreview,
		TargetFormat:  "zip",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Preview ZIP contains up to six JPG thumbnails sampled from the first seconds of the video",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-contact-sheet",
		DisplayName: "Generar contact sheet",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpPreview,
		TargetFormat:  "jpg",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Contact sheet shows up to six sampled frames arranged in a 3x2 grid",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-preview-mp4",
		DisplayName: "Generar preview corto MP4",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpPreview,
		TargetFormat:  "mp4",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Preview clip is capped to the first 8 seconds and rescaled for lightweight sharing",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-preview-webm",
		DisplayName: "Generar preview corto WebM",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpPreview,
		TargetFormat:  "webm",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 180,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Preview clip is capped to the first 8 seconds and rescaled for lightweight sharing",
		},
		Family: domain.FamilyVideo,
	},
	{
		ID:          "video-waveform-png",
		DisplayName: "Generar waveform PNG",
		SourceFormats: []string{
			"video/mp4", "video/quicktime",
			"video/webm", "video/x-msvideo",
		},
		OperationType: domain.OpPreview,
		TargetFormat:  "png",
		SizeLimits:    domain.SizeLimits{MaxInputBytes: mb500},
		ExecutionLimits: domain.ExecutionLimits{
			TimeoutSeconds: 120,
			MaxRetries:     1,
		},
		Engine:          "ffmpeg",
		ExpectedQuality: "good",
		KnownLimitations: []string{
			"Waveform preview summarizes only the primary audio stream and videos without audio will fail",
		},
		Family: domain.FamilyVideo,
	},
})

// ByID returns a capability by its ID, or nil if not found.
func ByID(id string) *domain.Capability {
	for i := range Catalog {
		if Catalog[i].ID == id {
			return &Catalog[i]
		}
	}
	return nil
}
