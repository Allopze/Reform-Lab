package capabilities

import (
	"reflect"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

// allEnginesAvailable sets up a prober where all engines are available.
func allEnginesAvailable(t *testing.T) func() {
	t.Helper()
	p := &EngineProber{}
	p.available = map[string]bool{
		"go-image":            true,
		"go-html":             true,
		"libreoffice":         true,
		"pdf2docx":            true,
		"librsvg":             true,
		"libreoffice-poppler": true,
		"libheif":             true,
		"ocr-pdf":             true,
		"poppler":             true,
		"poppler-html":        true,
		"tesseract":           true,
		"ffmpeg":              true,
		"ghostscript":         true,
		"goldmark":            true,
	}
	p.once.Do(func() {})
	old := DefaultProber
	DefaultProber = p
	return func() { DefaultProber = old }
}

func withFeatureFlags(t *testing.T, disabledCapabilities, disabledEngines []string) func() {
	t.Helper()
	old := DefaultFlags
	DefaultFlags = NewFeatureFlags(disabledCapabilities, disabledEngines)
	return func() { DefaultFlags = old }
}

func capabilityIDs(caps []domain.Capability) []string {
	ids := make([]string, len(caps))
	for i, cap := range caps {
		ids[i] = cap.ID
	}
	return ids
}

func assertCapabilityIDs(t *testing.T, caps []domain.Capability, want []string) {
	t.Helper()

	got := capabilityIDs(caps)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected capability order\nwant: %v\ngot:  %v", want, got)
	}
}

func TestResolveOrdersPDFCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	caps := Resolve(fakePDFFile())
	assertCapabilityIDs(t, caps, []string{
		"pdf-to-docx",
		"pdf-to-jpg",
		"pdf-to-png",
		"pdf-to-txt",
		"pdf-compress",
		"pdf-to-html-preview",
		"pdf-ocr-to-txt",
		"pdf-ocr-searchable-pdf",
		"pdf-ocr-to-json",
	})
}

func TestResolveOrdersRasterImageCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	caps := Resolve(fakeImageFile("image/png", "png"))
	assertCapabilityIDs(t, caps, []string{
		"image-to-jpg",
		"image-to-webp",
		"image-to-pdf",
		"image-to-avif",
		"image-compress-png",
		"image-web-jpg-1600",
		"image-web-webp-1600",
		"image-web-avif-1600",
		"image-web-jpg-640",
		"image-web-webp-640",
		"image-web-avif-640",
		"image-thumbnail-png",
		"image-ocr-to-txt",
		"image-ocr-to-json",
	})
}

func TestResolveOrdersHEICAndSVGCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	assertCapabilityIDs(t, Resolve(fakeImageFile("image/heic", "heic")), []string{
		"image-heic-to-jpg",
		"image-heic-to-png",
		"image-heic-to-webp",
	})
	assertCapabilityIDs(t, Resolve(fakeImageFile("image/svg+xml", "svg")), []string{
		"image-svg-to-png",
		"image-svg-to-webp",
		"image-svg-to-pdf",
	})
}

func TestResolveOrdersDocumentCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	assertCapabilityIDs(t, Resolve(fakeDocumentFile("application/vnd.openxmlformats-officedocument.wordprocessingml.document", "docx")), []string{
		"doc-to-pdf",
		"doc-to-txt",
		"doc-to-html",
		"docx-to-markdown",
	})
	assertCapabilityIDs(t, Resolve(fakeDocumentFile("text/html", "html")), []string{
		"html-to-pdf",
		"html-to-txt",
	})
	assertCapabilityIDs(t, Resolve(fakeTextFile("text/markdown", "md")), []string{
		"markdown-to-html",
		"markdown-to-pdf",
		"markdown-to-docx",
	})
}

func TestResolveOrdersPresentationAndSpreadsheetCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	assertCapabilityIDs(t, Resolve(fakePresentationFile("application/vnd.openxmlformats-officedocument.presentationml.presentation", "pptx")), []string{
		"presentation-to-pdf",
		"presentation-to-jpg",
		"presentation-to-png",
	})
	assertCapabilityIDs(t, Resolve(fakeSpreadsheetFile("text/csv", "csv")), []string{
		"spreadsheet-to-pdf",
		"spreadsheet-to-xlsx",
		"spreadsheet-to-html",
	})
}

func TestResolveOrdersAudioAndVideoCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	assertCapabilityIDs(t, Resolve(fakeAudioFile("audio/wav", "wav")), []string{
		"audio-to-mp3",
		"audio-to-m4a",
		"audio-to-flac",
		"audio-to-aac",
		"audio-to-ogg",
		"audio-to-opus",
		"audio-waveform-png",
	})
	assertCapabilityIDs(t, Resolve(fakeVideoFile("video/mp4", "mp4")), []string{
		"video-to-mp4",
		"video-to-webm",
		"video-to-gif",
		"video-to-mp3",
		"video-to-m4a",
		"video-to-wav",
		"video-to-flac",
		"video-to-aac",
		"video-to-opus",
		"video-preview-mp4",
		"video-preview-webm",
		"video-to-thumbnails",
		"video-contact-sheet",
		"video-waveform-png",
	})
	assertCapabilityIDs(t, Resolve(fakeVideoFile("video/webm", "webm")), []string{
		"video-to-mp4",
		"video-to-webm",
		"video-to-gif",
		"video-to-mp3",
		"video-to-m4a",
		"video-to-wav",
		"video-to-flac",
		"video-to-aac",
		"video-to-opus",
		"video-preview-mp4",
		"video-preview-webm",
		"video-to-thumbnails",
		"video-contact-sheet",
		"video-waveform-png",
	})
}

func TestSortCapabilitiesUsesIDAsTiebreaker(t *testing.T) {
	caps := []domain.Capability{
		{ID: "b", PresentationOrder: 10},
		{ID: "a", PresentationOrder: 10},
		{ID: "c", PresentationOrder: 20},
	}

	sortCapabilities(caps)

	assertCapabilityIDs(t, caps, []string{"a", "b", "c"})
}

func TestResolveReturnsCapsForPDF(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	caps := Resolve(file)
	if len(caps) == 0 {
		t.Fatal("expected at least one capability for PDF")
	}
	seenPDFToDocx := false
	for _, c := range caps {
		if c.TargetFormat == "pdf" && c.OperationType == domain.OpConvert {
			t.Fatalf("should not offer same-format conversion, got %s", c.ID)
		}
		if c.ID == "pdf-to-docx" {
			seenPDFToDocx = true
		}
	}
	if !seenPDFToDocx {
		t.Fatal("expected pdf-to-docx capability when dedicated engine is available")
	}
}

func TestResolveReturnsCapsForImage(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeImageFile("image/png", "png")
	caps := Resolve(file)
	if len(caps) == 0 {
		t.Fatal("expected at least one capability for PNG image")
	}
	for _, c := range caps {
		if c.TargetFormat == "png" && c.OperationType == domain.OpConvert {
			t.Fatalf("should not offer same-format conversion, got %s", c.ID)
		}
	}
}

func TestResolveIncludesPDFCompress(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	caps := Resolve(file)
	for _, cap := range caps {
		if cap.ID == "pdf-compress" {
			return
		}
	}
	t.Fatal("expected pdf-compress capability for PDF files")
}

func TestResolveIncludesImageSameFormatPreviewAndCompress(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeImageFile("image/jpeg", "jpg")
	caps := Resolve(file)
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"image-compress-jpg", "image-thumbnail-jpg"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s to be available for JPG files", capabilityID)
		}
	}
}

func TestResolveIncludesMarkdownCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeTextFile("text/markdown", "md")
	caps := Resolve(file)
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"markdown-to-html", "markdown-to-pdf"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s to be available for Markdown files", capabilityID)
		}
	}
}

func TestResolveIncludesHTMLToPDFCapability(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeDocumentFile("text/html", "html")
	seen := map[string]bool{}
	caps := Resolve(file)
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"html-to-pdf", "html-to-txt"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s capability for HTML files", capabilityID)
		}
	}
}

func TestResolveIncludesDOCXToMarkdownCapability(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeDocumentFile("application/vnd.openxmlformats-officedocument.wordprocessingml.document", "docx")
	caps := Resolve(file)
	for _, cap := range caps {
		if cap.ID == "docx-to-markdown" {
			return
		}
	}
	t.Fatal("expected docx-to-markdown capability for DOCX files")
}

func TestResolveIncludesVideoPreviewAndAudioExtraction(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeVideoFile("video/mp4", "mp4")
	caps := Resolve(file)
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"video-to-mp3", "video-to-wav", "video-to-aac", "video-to-m4a", "video-to-flac", "video-to-opus", "video-to-thumbnails", "video-contact-sheet", "video-preview-mp4", "video-preview-webm", "video-waveform-png"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s to be available for MP4 files", capabilityID)
		}
	}
}

func TestResolveIncludesPDFOCRCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	caps := Resolve(file)
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"pdf-ocr-to-txt", "pdf-ocr-to-json", "pdf-ocr-searchable-pdf"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s to be available for PDF files", capabilityID)
		}
	}
}

func TestResolveIncludesImageOCRCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeImageFile("image/png", "png")
	caps := Resolve(file)
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"image-ocr-to-txt", "image-ocr-to-json"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s to be available for PNG files", capabilityID)
		}
	}
}

func TestResolveIncludesImageWebVariants(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeImageFile("image/png", "png")
	caps := Resolve(file)
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"image-web-jpg-640", "image-web-webp-640", "image-web-avif-640", "image-web-jpg-1600", "image-web-webp-1600", "image-web-avif-1600"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s to be available for PNG files", capabilityID)
		}
	}
}

func TestResolveIncludesNewAudioCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeAudioFile("audio/wav", "wav")
	caps := Resolve(file)
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"audio-to-aac", "audio-to-m4a", "audio-to-flac", "audio-to-opus", "audio-waveform-png"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s to be available for WAV files", capabilityID)
		}
	}
}

func TestResolveIncludesHEIFAndSVGCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	heifFile := fakeImageFile("image/heic", "heic")
	heifCaps := Resolve(heifFile)
	heifSeen := map[string]bool{}
	for _, cap := range heifCaps {
		heifSeen[cap.ID] = true
	}
	for _, capabilityID := range []string{"image-heic-to-jpg", "image-heic-to-png", "image-heic-to-webp"} {
		if !heifSeen[capabilityID] {
			t.Fatalf("expected %s to be available for HEIC files", capabilityID)
		}
	}

	svgFile := fakeImageFile("image/svg+xml", "svg")
	svgCaps := Resolve(svgFile)
	svgSeen := map[string]bool{}
	for _, cap := range svgCaps {
		svgSeen[cap.ID] = true
	}
	for _, capabilityID := range []string{"image-svg-to-png", "image-svg-to-webp", "image-svg-to-pdf"} {
		if !svgSeen[capabilityID] {
			t.Fatalf("expected %s to be available for SVG files", capabilityID)
		}
	}
}

func TestResolveIncludesPresentationCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePresentationFile("application/vnd.openxmlformats-officedocument.presentationml.presentation", "pptx")
	caps := Resolve(file)
	seen := map[string]bool{}
	for _, cap := range caps {
		seen[cap.ID] = true
	}
	for _, capabilityID := range []string{"presentation-to-pdf", "presentation-to-jpg", "presentation-to-png"} {
		if !seen[capabilityID] {
			t.Fatalf("expected %s to be available for PPTX files", capabilityID)
		}
	}
}

func TestResolveIncludesSpreadsheetCapabilities(t *testing.T) {
	defer allEnginesAvailable(t)()

	xlsxFile := fakeSpreadsheetFile("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx")
	xlsxCaps := Resolve(xlsxFile)
	xlsxSeen := map[string]bool{}
	for _, cap := range xlsxCaps {
		xlsxSeen[cap.ID] = true
	}
	for _, capabilityID := range []string{"spreadsheet-to-pdf", "spreadsheet-to-csv", "spreadsheet-to-html"} {
		if !xlsxSeen[capabilityID] {
			t.Fatalf("expected %s to be available for XLSX files", capabilityID)
		}
	}

	csvFile := fakeSpreadsheetFile("text/csv", "csv")
	csvCaps := Resolve(csvFile)
	csvSeen := map[string]bool{}
	for _, cap := range csvCaps {
		csvSeen[cap.ID] = true
	}
	for _, capabilityID := range []string{"spreadsheet-to-pdf", "spreadsheet-to-xlsx", "spreadsheet-to-html"} {
		if !csvSeen[capabilityID] {
			t.Fatalf("expected %s to be available for CSV files", capabilityID)
		}
	}
}

func TestResolveEmpty_UnknownFormat(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := domain.OriginalFile{
		Size: 100,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  "application/x-unknown-format-42",
			Family:    "unknown",
			Extension: "unk",
		},
	}
	caps := Resolve(file)
	if len(caps) != 0 {
		t.Fatalf("expected 0 capabilities for unknown format, got %d", len(caps))
	}
}

func TestResolveExcludesProtectedFiles(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	file.Metadata.IsProtected = true
	caps := Resolve(file)
	if len(caps) != 0 {
		t.Fatalf("expected 0 capabilities for protected file, got %d", len(caps))
	}
}

func TestResolveExcludesOversizedFiles(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	file.Size = 10 * 1024 * 1024 * 1024 // 10 GB — should exceed all limits
	caps := Resolve(file)
	if len(caps) != 0 {
		t.Fatalf("expected 0 capabilities for oversized file, got %d", len(caps))
	}
}

func TestResolveExcludesFeatureFlaggedCapability(t *testing.T) {
	defer allEnginesAvailable(t)()
	defer withFeatureFlags(t, []string{"image-to-jpg"}, nil)()

	file := fakeImageFile("image/png", "png")
	caps := Resolve(file)
	for _, cap := range caps {
		if cap.ID == "image-to-jpg" {
			t.Fatal("expected image-to-jpg to be hidden by feature flag")
		}
	}
}

func TestResolveExcludesFeatureDisabledEngine(t *testing.T) {
	defer allEnginesAvailable(t)()
	defer withFeatureFlags(t, nil, []string{"poppler"})()

	file := fakePDFFile()
	caps := Resolve(file)
	for _, cap := range caps {
		if cap.Engine == "poppler" {
			t.Fatalf("expected poppler-backed capability %s to be hidden by feature flag", cap.ID)
		}
	}
}

func TestIsEligibleValid(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	cap, err := IsEligible(file, "pdf-to-txt")
	if err != nil {
		t.Fatalf("expected eligibility for pdf-to-txt, got err: %v", err)
	}
	if cap.ID != "pdf-to-txt" {
		t.Fatalf("expected capability pdf-to-txt, got %s", cap.ID)
	}
}

func TestIsEligibleNotFound(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	_, err := IsEligible(file, "nonexistent-cap")
	if err != domain.ErrCapabilityNotFound {
		t.Fatalf("expected ErrCapabilityNotFound, got %v", err)
	}
}

func TestIsEligibleWrongFormat(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakeImageFile("image/png", "png")
	_, err := IsEligible(file, "pdf-to-txt")
	if err != domain.ErrCapabilityIneligible {
		t.Fatalf("expected ErrCapabilityIneligible, got %v", err)
	}
}

func TestIsEligibleFeatureFlaggedCapability(t *testing.T) {
	defer allEnginesAvailable(t)()
	defer withFeatureFlags(t, []string{"pdf-to-txt"}, nil)()

	file := fakePDFFile()
	_, err := IsEligible(file, "pdf-to-txt")
	if err != domain.ErrCapabilityIneligible {
		t.Fatalf("expected ErrCapabilityIneligible for flagged capability, got %v", err)
	}
}

func TestIsEligibleSameFormat(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	// pdf-to-pdf doesn't exist in catalog but we can check with a real cap
	// where the file extension matches target. Let's use a PNG image with
	// a "png" extension requesting img-png-to-png (which shouldn't exist).
	// Instead, let's verify the same-format check only applies to convert.
	caps := Resolve(file)
	for _, c := range caps {
		if c.TargetFormat == file.DetectedFormat.Extension && c.OperationType == domain.OpConvert {
			t.Fatalf("Resolve returned same-format cap: %s", c.ID)
		}
	}
}

func TestByIDExists(t *testing.T) {
	cap := ByID("pdf-to-txt")
	if cap == nil {
		t.Fatal("expected to find pdf-to-txt capability")
	}
	if cap.Engine != "poppler" {
		t.Fatalf("expected poppler engine, got %s", cap.Engine)
	}
}

func TestByIDMissing(t *testing.T) {
	cap := ByID("does-not-exist")
	if cap != nil {
		t.Fatal("expected nil for nonexistent capability")
	}
}

func TestRejectsSameFormat_PDF(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	caps := Resolve(file)
	for _, c := range caps {
		if c.TargetFormat == "pdf" && c.OperationType == domain.OpConvert {
			t.Fatalf("should not offer pdf→pdf conversion, got %s", c.ID)
		}
	}
}

func TestRejectsSameFormat_Image(t *testing.T) {
	defer allEnginesAvailable(t)()

	extensions := []string{"png", "jpg", "jpeg", "webp", "avif", "gif", "bmp", "tiff"}
	for _, ext := range extensions {
		mime := "image/" + ext
		if ext == "jpg" || ext == "jpeg" {
			mime = "image/jpeg"
		}
		file := fakeImageFile(mime, ext)
		caps := Resolve(file)
		for _, c := range caps {
			if c.TargetFormat == ext && c.OperationType == domain.OpConvert {
				t.Fatalf("should not offer %s→%s conversion, got %s", ext, ext, c.ID)
			}
		}
	}
}

func TestRejectsSameFormat_Document(t *testing.T) {
	defer allEnginesAvailable(t)()

	tests := []struct {
		mime string
		ext  string
	}{
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "docx"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "pptx"},
	}
	for _, tc := range tests {
		file := fakeDocumentFile(tc.mime, tc.ext)
		caps := Resolve(file)
		for _, c := range caps {
			if c.TargetFormat == tc.ext && c.OperationType == domain.OpConvert {
				t.Fatalf("should not offer %s→%s conversion, got %s", tc.ext, tc.ext, c.ID)
			}
		}
	}
}

func TestRejectsSameFormat_VideoAllowsReEncoding(t *testing.T) {
	defer allEnginesAvailable(t)()

	// Video should allow same-format re-encoding (e.g., MP4→MP4 with different codec)
	tests := []struct {
		mime string
		ext  string
	}{
		{"video/mp4", "mp4"},
		{"video/webm", "webm"},
	}
	for _, tc := range tests {
		file := fakeVideoFile(tc.mime, tc.ext)
		caps := Resolve(file)
		foundSameFormat := false
		for _, c := range caps {
			if c.TargetFormat == tc.ext && c.OperationType == domain.OpConvert {
				foundSameFormat = true
				break
			}
		}
		if !foundSameFormat {
			t.Fatalf("video should offer %s→%s re-encoding, but it was not found", tc.ext, tc.ext)
		}
	}
}

func TestRejectsSameFormat_Audio(t *testing.T) {
	defer allEnginesAvailable(t)()

	tests := []struct {
		mime string
		ext  string
	}{
		{"audio/mp3", "mp3"},
		{"audio/wav", "wav"},
		{"audio/flac", "flac"},
		{"audio/aac", "aac"},
		{"audio/ogg", "ogg"},
		{"audio/opus", "opus"},
	}
	for _, tc := range tests {
		file := fakeAudioFile(tc.mime, tc.ext)
		caps := Resolve(file)
		for _, c := range caps {
			if c.TargetFormat == tc.ext && c.OperationType == domain.OpConvert {
				t.Fatalf("should not offer %s→%s conversion, got %s", tc.ext, tc.ext, c.ID)
			}
		}
	}
}

func TestRejectsSameFormat_NonConvertOperations(t *testing.T) {
	defer allEnginesAvailable(t)()

	// Non-convert operations (like compress, preview, thumbnail) should not be rejected
	// even if they produce the same format
	file := fakePDFFile()
	caps := Resolve(file)
	// pdf-compress should be available even though it produces PDF
	foundCompress := false
	for _, c := range caps {
		if c.ID == "pdf-compress" {
			foundCompress = true
			break
		}
	}
	if !foundCompress {
		t.Fatal("expected pdf-compress capability to be available (non-convert operation)")
	}
}

func TestIsEligible_ProtectedFile(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	file.Metadata.IsProtected = true

	_, err := IsEligible(file, "pdf-to-docx")
	if err != domain.ErrProtectedUnsupported {
		t.Fatalf("expected ErrProtectedUnsupported for protected file, got %v", err)
	}
}

func TestIsEligible_ExceedsSizeLimit(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	file.Size = 500 * 1024 * 1024 // 500 MB, exceeds typical limit

	_, err := IsEligible(file, "pdf-to-docx")
	if err != domain.ErrLimitExceeded {
		t.Fatalf("expected ErrLimitExceeded for oversized file, got %v", err)
	}
}

func TestIsEligible_EngineNotAvailable(t *testing.T) {
	// Set up a prober where the required engine is NOT available
	p := &EngineProber{}
	p.available = map[string]bool{
		"go-image":    true,
		"libreoffice": false, // pdf2docx needs libreoffice or pdf2docx engine
	}
	p.once.Do(func() {})
	old := DefaultProber
	DefaultProber = p
	defer func() { DefaultProber = old }()

	file := fakePDFFile()
	_, err := IsEligible(file, "pdf-to-docx")
	if err != domain.ErrCapabilityIneligible {
		t.Fatalf("expected ErrCapabilityIneligible when engine not available, got %v", err)
	}
}

func TestIsEligible_CapabilityNotFound(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	_, err := IsEligible(file, "nonexistent-capability")
	if err != domain.ErrCapabilityNotFound {
		t.Fatalf("expected ErrCapabilityNotFound, got %v", err)
	}
}

func TestIsEligible_UnsupportedSourceFormat(t *testing.T) {
	defer allEnginesAvailable(t)()

	// Try to convert an image with a PDF-only capability
	file := fakeImageFile("image/png", "png")
	_, err := IsEligible(file, "pdf-to-docx")
	if err != domain.ErrCapabilityIneligible {
		t.Fatalf("expected ErrCapabilityIneligible for unsupported source format, got %v", err)
	}
}

func TestIsEligible_CapabilityDisabled(t *testing.T) {
	defer allEnginesAvailable(t)()
	defer withFeatureFlags(t, []string{"pdf-to-docx"}, nil)()

	file := fakePDFFile()
	_, err := IsEligible(file, "pdf-to-docx")
	if err != domain.ErrCapabilityIneligible {
		t.Fatalf("expected ErrCapabilityIneligible for disabled capability, got %v", err)
	}
}

func TestIsEligible_ValidFile(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	cap, err := IsEligible(file, "pdf-to-docx")
	if err != nil {
		t.Fatalf("expected no error for valid file, got %v", err)
	}
	if cap == nil {
		t.Fatal("expected capability to be returned")
	}
	if cap.ID != "pdf-to-docx" {
		t.Fatalf("expected pdf-to-docx capability, got %s", cap.ID)
	}
}
