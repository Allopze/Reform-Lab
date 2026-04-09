package pdf

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/workers/document"
)

func TestOCRToTextEngineExtractsTextFromPDF(t *testing.T) {
	for _, bin := range []string{"libreoffice", "pdftoppm", "tesseract"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " not available")
		}
	}

	dir := t.TempDir()
	textPath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(textPath, []byte("HELLO OCR PDF\n"), 0o644); err != nil {
		t.Fatalf("write text source: %v", err)
	}
	pdfPath, err := (&document.ToPDFEngine{}).Execute(context.Background(), textPath, dir, "pdf")
	if err != nil {
		t.Fatalf("prepare pdf: %v", err)
	}

	outputPath, err := (&OCRToTextEngine{}).Execute(context.Background(), pdfPath, dir, "txt")
	if err != nil {
		t.Fatalf("pdf ocr to txt: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read ocr text: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(data)), "hello") {
		t.Fatalf("expected OCR text to contain hello, got %q", data)
	}
}

func TestOCRToSearchablePDFEngineCreatesPDF(t *testing.T) {
	for _, bin := range []string{"libreoffice", "pdftoppm", "tesseract", "gs"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " not available")
		}
	}

	dir := t.TempDir()
	textPath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(textPath, []byte("HELLO OCR PDF\n"), 0o644); err != nil {
		t.Fatalf("write text source: %v", err)
	}
	pdfPath, err := (&document.ToPDFEngine{}).Execute(context.Background(), textPath, dir, "pdf")
	if err != nil {
		t.Fatalf("prepare pdf: %v", err)
	}

	outputPath, err := (&OCRToSearchablePDFEngine{}).Execute(context.Background(), pdfPath, dir, "pdf")
	if err != nil {
		t.Fatalf("pdf ocr searchable pdf: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read searchable pdf: %v", err)
	}
	if !strings.HasPrefix(string(data[:5]), "%PDF-") {
		t.Fatalf("expected pdf header, got %q", data[:min(len(data), 5)])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
