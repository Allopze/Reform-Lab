package workers

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

func TestValidateOutputArtifactRejectsInvalidJSON(t *testing.T) {
	path := writeValidationFile(t, "broken.json", []byte(`{"ok":true`))

	if _, err := validateOutputArtifact(path, "json", 1024); err == nil {
		t.Fatal("expected invalid JSON output to be rejected")
	}
}

func TestValidateOutputArtifactAcceptsSingleRowCSV(t *testing.T) {
	path := writeValidationFile(t, "single-row.csv", []byte("only-one-cell\n"))

	if _, err := validateOutputArtifact(path, "csv", 1024); err != nil {
		t.Fatalf("expected single-row CSV output to be accepted: %v", err)
	}
}

func TestValidateOutputArtifactRejectsBinaryTextOutput(t *testing.T) {
	path := writeValidationFile(t, "bad.txt", []byte{0x41, 0x00, 0x42})

	if _, err := validateOutputArtifact(path, "txt", 1024); err == nil {
		t.Fatal("expected binary text output to be rejected")
	}
}

func TestValidateOutputArtifactRejectsMismatchedBinaryFormat(t *testing.T) {
	path := writeValidationFile(t, "image.png", minimalPNG())

	if _, err := validateOutputArtifact(path, "jpg", 1024); err == nil {
		t.Fatal("expected mismatched image output to be rejected")
	}
}

func TestValidateOutputArtifactAcceptsHTMLWithCharsetMIME(t *testing.T) {
	path := writeValidationFile(t, "page.html", []byte("<html><body>ok</body></html>"))

	if _, err := validateOutputArtifact(path, "html", 1024); err != nil {
		t.Fatalf("expected html output to be accepted: %v", err)
	}
}

func TestValidateOutputArtifactValidatesZipOutputs(t *testing.T) {
	validZip := writeZipBytesValidationFile(t, "preview.zip", map[string][]byte{
		"frame-001.jpg": minimalJPEG(),
	})
	if _, err := validateOutputArtifact(validZip, "zip", 1024); err != nil {
		t.Fatalf("expected non-empty zip output to be accepted: %v", err)
	}

	htmlZip := writeZipValidationFile(t, "html-export.zip", map[string]string{
		"index.html": "<html><body>ok</body></html>",
	})
	if _, err := validateOutputArtifact(htmlZip, "zip", 1024); err != nil {
		t.Fatalf("expected html package zip output to be accepted: %v", err)
	}

	emptyZip := writeZipValidationFile(t, "empty.zip", nil)
	if _, err := validateOutputArtifact(emptyZip, "zip", 1024); err == nil {
		t.Fatal("expected empty zip output to be rejected")
	}

	traversalZip := writeZipValidationFile(t, "traversal.zip", map[string]string{
		"../outside.txt": "nope",
	})
	if _, err := validateOutputArtifact(traversalZip, "zip", 1024); err == nil {
		t.Fatal("expected zip path traversal to be rejected")
	}

	emptyFileZip := writeZipValidationFile(t, "empty-file.zip", map[string]string{
		"frame-001.jpg": "",
	})
	if _, err := validateOutputArtifact(emptyFileZip, "zip", 1024); err == nil {
		t.Fatal("expected zip with empty file to be rejected")
	}

	corruptImageZip := writeZipValidationFile(t, "corrupt-image.zip", map[string]string{
		"frame-001.jpg": "fake-image-data",
	})
	if _, err := validateOutputArtifact(corruptImageZip, "zip", 1024); err == nil {
		t.Fatal("expected zip with corrupt image entry to be rejected")
	}
}

func TestValidateOutputArtifactValidatesOOXMLOutputs(t *testing.T) {
	docxPath := writeZipValidationFile(t, "document.docx", map[string]string{
		"[Content_Types].xml": "<Types/>",
		"word/document.xml":   "<document/>",
	})
	if _, err := validateOutputArtifact(docxPath, "docx", 1024); err != nil {
		t.Fatalf("expected docx output to be accepted: %v", err)
	}

	xlsxPath := writeZipValidationFile(t, "sheet.xlsx", map[string]string{
		"[Content_Types].xml": "<Types/>",
		"xl/workbook.xml":     "<workbook/>",
	})
	if _, err := validateOutputArtifact(xlsxPath, "xlsx", 1024); err != nil {
		t.Fatalf("expected xlsx output to be accepted: %v", err)
	}

	brokenDocxPath := writeZipValidationFile(t, "broken.docx", map[string]string{
		"[Content_Types].xml": "<Types/>",
	})
	if _, err := validateOutputArtifact(brokenDocxPath, "docx", 1024); err == nil {
		t.Fatal("expected broken docx output to be rejected")
	}
}

func TestValidateOutputArtifactRejectsTruncatedPDF(t *testing.T) {
	path := writeValidationFile(t, "truncated.pdf", []byte("%PDF-1.7\n1 0 obj<<>>endobj\n"))

	if _, err := validateOutputArtifact(path, "pdf", 1024); err == nil {
		t.Fatal("expected truncated PDF output to be rejected")
	}
}

func TestValidateOutputArtifactAcceptsPDFWithEOFPastInitialSample(t *testing.T) {
	data := make([]byte, outputValidationSampleLimit+1024)
	copy(data, []byte("%PDF-1.7\n"))
	copy(data[64:], []byte("1 0 obj<<>>endobj\n"))
	tail := []byte("\nstartxref\n0\n%%EOF\n")
	copy(data[len(data)-len(tail):], tail)
	path := writeValidationFile(t, "large-valid.pdf", data)

	if _, err := validateOutputArtifact(path, "pdf", int64(len(data))); err != nil {
		t.Fatalf("expected PDF with EOF near file tail to be accepted: %v", err)
	}
}

func TestValidateOutputArtifactAllowsSmallExtractOutputs(t *testing.T) {
	path := writeValidationFile(t, "small.txt", []byte("ok text\n"))

	if _, err := validateOutputArtifact(path, "txt", 10*1024*1024, domain.OpExtract); err != nil {
		t.Fatalf("expected small extraction output to be accepted: %v", err)
	}
}

func TestValidateOutputArtifactAllowsSmallPreviewOutputs(t *testing.T) {
	path := writeValidationFile(t, "preview.png", minimalPNG())

	if _, err := validateOutputArtifact(path, "png", 10*1024*1024, domain.OpPreview); err != nil {
		t.Fatalf("expected small preview output to be accepted: %v", err)
	}
}

func TestValidateOutputArtifactAllowsSmallOptimizeOutputs(t *testing.T) {
	path := writeValidationFile(t, "optimized.png", minimalPNG())

	if _, err := validateOutputArtifact(path, "png", 10*1024*1024, domain.OpOptimize); err != nil {
		t.Fatalf("expected small optimized output to be accepted: %v", err)
	}
}

func TestValidateOutputArtifactStillRejectsTinyConvertOutputs(t *testing.T) {
	path := writeValidationFile(t, "converted.png", minimalPNG())

	if _, err := validateOutputArtifact(path, "png", 10*1024*1024, domain.OpConvert); err == nil {
		t.Fatal("expected tiny convert output to remain suspicious")
	}
}

func writeValidationFile(t *testing.T, name string, data []byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write validation file: %v", err)
	}
	return path
}

func writeZipValidationFile(t *testing.T, name string, entries map[string]string) string {
	t.Helper()

	byteEntries := make(map[string][]byte, len(entries))
	for entryName, content := range entries {
		byteEntries[entryName] = []byte(content)
	}
	return writeZipBytesValidationFile(t, name, byteEntries)
}

func writeZipBytesValidationFile(t *testing.T, name string, entries map[string][]byte) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip output: %v", err)
	}

	writer := zip.NewWriter(file)
	for entryName, content := range entries {
		entry, err := writer.Create(entryName)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", entryName, err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatalf("write zip entry %s: %v", entryName, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	return path
}

func minimalPNG() []byte {
	return []byte{
		0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n',
		0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00,
		0x90, 0x77, 0x53, 0xde,
		0x00, 0x00, 0x00, 0x0a, 'I', 'D', 'A', 'T',
		0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00, 0x00, 0x03, 0x01, 0x01, 0x00,
		0x18, 0xdd, 0x8d, 0xb1,
		0x00, 0x00, 0x00, 0x00, 'I', 'E', 'N', 'D', 0xae, 'B', 0x60, 0x82,
	}
}

func minimalJPEG() []byte {
	return []byte{
		0xff, 0xd8,
		0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00,
		0xff, 0xdb, 0x00, 0x43, 0x00,
		0xff, 0xc0, 0x00, 0x11, 0x08, 0x00, 0x01, 0x00, 0x01, 0x03, 0x01, 0x11, 0x00, 0x02, 0x11, 0x01, 0x03, 0x11, 0x01,
		0xff, 0xd9,
	}
}

func TestNormalizeOutputMIME(t *testing.T) {
	tests := map[string]string{
		"application/x-pdf":        "application/pdf",
		"text/html; charset=utf-8": "text/html",
		"audio/x-wav":              "audio/wav",
		"audio/mp3":                "audio/mpeg",
		"application/x-zip":        "application/zip",
		"application/octet-stream": "application/octet-stream",
	}

	for input, want := range tests {
		if got := normalizeOutputMIME(input); got != want {
			t.Fatalf("normalizeOutputMIME(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidateOutputArtifactRejectsUnknownBinaryFormat(t *testing.T) {
	// Create a file large enough to pass size validation but with wrong MIME type
	data := make([]byte, 300)
	copy(data, []byte("<html><body>ok</body></html>"))
	path := writeValidationFile(t, "note.html", data)

	_, err := validateOutputArtifact(path, "mp4", 1024)
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected mismatch error for wrong binary format, got %v", err)
	}
}
