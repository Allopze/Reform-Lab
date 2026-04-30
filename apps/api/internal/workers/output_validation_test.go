package workers

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateOutputArtifactRejectsInvalidJSON(t *testing.T) {
	path := writeValidationFile(t, "broken.json", []byte(`{"ok":true`))

	if _, err := validateOutputArtifact(path, "json"); err == nil {
		t.Fatal("expected invalid JSON output to be rejected")
	}
}

func TestValidateOutputArtifactAcceptsSingleRowCSV(t *testing.T) {
	path := writeValidationFile(t, "single-row.csv", []byte("only-one-cell\n"))

	if _, err := validateOutputArtifact(path, "csv"); err != nil {
		t.Fatalf("expected single-row CSV output to be accepted: %v", err)
	}
}

func TestValidateOutputArtifactRejectsBinaryTextOutput(t *testing.T) {
	path := writeValidationFile(t, "bad.txt", []byte{0x41, 0x00, 0x42})

	if _, err := validateOutputArtifact(path, "txt"); err == nil {
		t.Fatal("expected binary text output to be rejected")
	}
}

func TestValidateOutputArtifactRejectsMismatchedBinaryFormat(t *testing.T) {
	path := writeValidationFile(t, "image.png", minimalPNG())

	if _, err := validateOutputArtifact(path, "jpg"); err == nil {
		t.Fatal("expected mismatched image output to be rejected")
	}
}

func TestValidateOutputArtifactValidatesZipOutputs(t *testing.T) {
	validZip := writeZipValidationFile(t, "preview.zip", map[string]string{
		"frame-001.jpg": "fake-image-data",
	})
	if _, err := validateOutputArtifact(validZip, "zip"); err != nil {
		t.Fatalf("expected non-empty zip output to be accepted: %v", err)
	}

	emptyZip := writeZipValidationFile(t, "empty.zip", nil)
	if _, err := validateOutputArtifact(emptyZip, "zip"); err == nil {
		t.Fatal("expected empty zip output to be rejected")
	}

	traversalZip := writeZipValidationFile(t, "traversal.zip", map[string]string{
		"../outside.txt": "nope",
	})
	if _, err := validateOutputArtifact(traversalZip, "zip"); err == nil {
		t.Fatal("expected zip path traversal to be rejected")
	}

	emptyFileZip := writeZipValidationFile(t, "empty-file.zip", map[string]string{
		"frame-001.jpg": "",
	})
	if _, err := validateOutputArtifact(emptyFileZip, "zip"); err == nil {
		t.Fatal("expected zip with empty file to be rejected")
	}
}

func TestValidateOutputArtifactValidatesOOXMLOutputs(t *testing.T) {
	docxPath := writeZipValidationFile(t, "document.docx", map[string]string{
		"[Content_Types].xml": "<Types/>",
		"word/document.xml":   "<document/>",
	})
	if _, err := validateOutputArtifact(docxPath, "docx"); err != nil {
		t.Fatalf("expected docx output to be accepted: %v", err)
	}

	xlsxPath := writeZipValidationFile(t, "sheet.xlsx", map[string]string{
		"[Content_Types].xml": "<Types/>",
		"xl/workbook.xml":     "<workbook/>",
	})
	if _, err := validateOutputArtifact(xlsxPath, "xlsx"); err != nil {
		t.Fatalf("expected xlsx output to be accepted: %v", err)
	}

	brokenDocxPath := writeZipValidationFile(t, "broken.docx", map[string]string{
		"[Content_Types].xml": "<Types/>",
	})
	if _, err := validateOutputArtifact(brokenDocxPath, "docx"); err == nil {
		t.Fatal("expected broken docx output to be rejected")
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
		if _, err := entry.Write([]byte(content)); err != nil {
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

func TestNormalizeOutputMIME(t *testing.T) {
	tests := map[string]string{
		"application/x-pdf":        "application/pdf",
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
	path := writeValidationFile(t, "note.html", []byte("<html><body>ok</body></html>"))

	_, err := validateOutputArtifact(path, "mp4")
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("expected mismatch error for wrong binary format, got %v", err)
	}
}
