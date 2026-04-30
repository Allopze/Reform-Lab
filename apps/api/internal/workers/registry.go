package workers

import (
	"context"
	"fmt"
	"sort"

	"github.com/allopze/reform-lab/apps/api/internal/workers/audio"
	"github.com/allopze/reform-lab/apps/api/internal/workers/document"
	workerImage "github.com/allopze/reform-lab/apps/api/internal/workers/image"
	"github.com/allopze/reform-lab/apps/api/internal/workers/pdf"
	"github.com/allopze/reform-lab/apps/api/internal/workers/video"
)

// Engine executes a conversion and produces an output file.
type Engine interface {
	// Execute runs the conversion from inputPath, writing results to outputDir.
	// Returns the path to the primary output file.
	Execute(ctx context.Context, inputPath string, outputDir string, outputFormat string) (outputPath string, err error)
}

// Registry maps capability IDs to their responsible engine.
type Registry struct {
	engines map[string]Engine
}

// NewRegistry creates an empty engine registry.
func NewRegistry() *Registry {
	return &Registry{engines: make(map[string]Engine)}
}

// Register associates a capability ID with an engine.
func (r *Registry) Register(capabilityID string, engine Engine) {
	r.engines[capabilityID] = engine
}

// IDs returns all registered capability IDs in stable order.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.engines))
	for id := range r.engines {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Get returns the engine for a capability, or an error if not found.
func (r *Registry) Get(capabilityID string) (Engine, error) {
	e, ok := r.engines[capabilityID]
	if !ok {
		return nil, fmt.Errorf("no engine registered for capability %q", capabilityID)
	}
	return e, nil
}

// BuildDefaultRegistry is the single runtime registry for conversion engines.
func BuildDefaultRegistry() *Registry {
	registry := NewRegistry()

	registry.Register("pdf-to-jpg", &pdf.ToImagesEngine{})
	registry.Register("pdf-to-png", &pdf.ToImagesEngine{})
	registry.Register("pdf-to-txt", &pdf.ToTextEngine{})
	registry.Register("pdf-to-docx", &pdf.ToDocxEngine{})
	registry.Register("pdf-compress", &pdf.CompressEngine{})
	registry.Register("pdf-to-html-preview", &pdf.ToHTMLEngine{})
	registry.Register("pdf-ocr-to-txt", &pdf.OCRToTextEngine{})
	registry.Register("pdf-ocr-to-json", &pdf.OCRToJSONEngine{})
	registry.Register("pdf-ocr-searchable-pdf", &pdf.OCRToSearchablePDFEngine{})

	imgConvert := &workerImage.ConvertEngine{}
	registry.Register("image-to-png", imgConvert)
	registry.Register("image-to-jpg", imgConvert)
	registry.Register("image-to-webp", &workerImage.ToWebPEngine{})
	registry.Register("image-to-avif", &workerImage.ToAVIFEngine{})
	registry.Register("image-to-pdf", &workerImage.ToPDFEngine{})
	registry.Register("image-heic-to-jpg", &workerImage.HEIFConvertEngine{})
	registry.Register("image-heic-to-png", &workerImage.HEIFConvertEngine{})
	registry.Register("image-heic-to-webp", &workerImage.HEIFConvertEngine{})
	registry.Register("image-svg-to-png", &workerImage.SVGConvertEngine{})
	registry.Register("image-svg-to-webp", &workerImage.SVGConvertEngine{})
	registry.Register("image-svg-to-pdf", &workerImage.SVGConvertEngine{})
	registry.Register("image-compress-jpg", &workerImage.CompressEngine{})
	registry.Register("image-compress-png", &workerImage.CompressEngine{})
	registry.Register("image-thumbnail-jpg", &workerImage.ThumbnailEngine{})
	registry.Register("image-thumbnail-png", &workerImage.ThumbnailEngine{})
	registry.Register("image-ocr-to-txt", &workerImage.OCRToTextEngine{})
	registry.Register("image-ocr-to-json", &workerImage.OCRToJSONEngine{})
	smallWebVariant := workerImage.NewWebVariantEngine(640)
	largeWebVariant := workerImage.NewWebVariantEngine(1600)
	registry.Register("image-web-jpg-640", smallWebVariant)
	registry.Register("image-web-webp-640", smallWebVariant)
	registry.Register("image-web-avif-640", smallWebVariant)
	registry.Register("image-web-jpg-1600", largeWebVariant)
	registry.Register("image-web-webp-1600", largeWebVariant)
	registry.Register("image-web-avif-1600", largeWebVariant)

	registry.Register("doc-to-pdf", &document.ToPDFEngine{})
	registry.Register("doc-to-txt", &document.ToTxtEngine{})
	registry.Register("doc-to-docx", &document.ToDocxEngine{})
	registry.Register("doc-to-html", &document.ToHTMLEngine{})
	registry.Register("docx-to-markdown", &document.DOCXToMarkdownEngine{})
	registry.Register("txt-to-pdf", &document.ToPDFEngine{})
	registry.Register("html-to-pdf", &document.ToPDFEngine{})
	registry.Register("html-to-txt", &document.HTMLToTextEngine{})
	registry.Register("markdown-to-html", &document.MarkdownToHTMLEngine{})
	registry.Register("markdown-to-pdf", &document.MarkdownToPDFEngine{})
	registry.Register("markdown-to-docx", &document.MarkdownToDocxEngine{})
	registry.Register("presentation-to-pdf", &document.ToPDFEngine{})
	registry.Register("presentation-to-jpg", &document.PresentationToImagesEngine{})
	registry.Register("presentation-to-png", &document.PresentationToImagesEngine{})
	registry.Register("spreadsheet-to-pdf", &document.ToPDFEngine{})
	registry.Register("spreadsheet-to-csv", &document.ToCSVEngine{})
	registry.Register("spreadsheet-to-xlsx", &document.ToXLSXEngine{})
	registry.Register("spreadsheet-to-html", &document.ToHTMLEngine{})

	audioConvert := &audio.ConvertEngine{}
	registry.Register("audio-to-mp3", audioConvert)
	registry.Register("audio-to-wav", audioConvert)
	registry.Register("audio-to-ogg", audioConvert)
	registry.Register("audio-to-aac", audioConvert)
	registry.Register("audio-to-m4a", audioConvert)
	registry.Register("audio-to-flac", audioConvert)
	registry.Register("audio-to-opus", audioConvert)
	registry.Register("video-to-mp3", audioConvert)
	registry.Register("video-to-wav", audioConvert)
	registry.Register("video-to-aac", audioConvert)
	registry.Register("video-to-m4a", audioConvert)
	registry.Register("video-to-flac", audioConvert)
	registry.Register("video-to-opus", audioConvert)
	waveform := &audio.WaveformEngine{}
	registry.Register("audio-waveform-png", waveform)
	registry.Register("video-waveform-png", waveform)

	videoConvert := &video.ConvertEngine{}
	registry.Register("video-to-mp4", videoConvert)
	registry.Register("video-to-webm", videoConvert)
	registry.Register("video-to-gif", &video.ToGIFEngine{})
	registry.Register("video-to-thumbnails", &video.ThumbnailZipEngine{})
	registry.Register("video-contact-sheet", &video.ContactSheetEngine{})
	registry.Register("video-preview-mp4", &video.PreviewClipEngine{})
	registry.Register("video-preview-webm", &video.PreviewClipEngine{})

	return registry
}

func DefaultCapabilityIDs() []string {
	return BuildDefaultRegistry().IDs()
}
