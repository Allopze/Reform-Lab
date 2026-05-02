package ingestion

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

func ingestionFixturePath(parts ...string) string {
	path := []string{"..", "..", "tests", "fixtures"}
	path = append(path, parts...)
	return filepath.Join(path...)
}

func TestDetectFormatRecognizesMarkdownHeuristically(t *testing.T) {
	source := []byte("# Title\n\n- item one\n- item two\n\n[link](https://example.com)\n")
	format, err := DetectFormat(bytes.NewReader(source))
	if err != nil {
		t.Fatalf("DetectFormat returned error: %v", err)
	}
	if format.MIMEType != "text/markdown" {
		t.Fatalf("expected text/markdown, got %s", format.MIMEType)
	}
	if format.Extension != "md" {
		t.Fatalf("expected md extension, got %s", format.Extension)
	}
	if format.Family != domain.FamilyDocument {
		t.Fatalf("expected document family, got %s", format.Family)
	}
}

func TestDetectFormatKeepsPlainTextWhenMarkdownSignalsAreWeak(t *testing.T) {
	source := []byte("Hola mundo\nEste es un archivo de texto simple.\nSin sintaxis estructurada.\n")
	format, err := DetectFormat(bytes.NewReader(source))
	if err != nil {
		t.Fatalf("DetectFormat returned error: %v", err)
	}
	if format.MIMEType != "text/plain" {
		t.Fatalf("expected text/plain, got %s", format.MIMEType)
	}
}

func TestDetectFormatRecognizesLargeRootSVG(t *testing.T) {
	source := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="4096" height="1024">` + strings.Repeat(`<path d="M 1 1 L 2 2"/>`, 5000) + `</svg>`)
	format, err := DetectFormat(bytes.NewReader(source))
	if err != nil {
		t.Fatalf("DetectFormat returned error: %v", err)
	}
	if format.MIMEType != "image/svg+xml" {
		t.Fatalf("expected image/svg+xml, got %s", format.MIMEType)
	}
	if format.Extension != "svg" {
		t.Fatalf("expected svg extension, got %s", format.Extension)
	}
	if format.Family != domain.FamilyImage {
		t.Fatalf("expected image family, got %s", format.Family)
	}
}

func TestDetectFormatDoesNotTreatEmbeddedSVGAsRootSVG(t *testing.T) {
	source := []byte(`<!doctype html><html><body><svg width="10" height="10"></svg></body></html>`)
	format, err := DetectFormat(bytes.NewReader(source))
	if err != nil {
		t.Fatalf("DetectFormat returned error: %v", err)
	}
	if format.MIMEType == "image/svg+xml" {
		t.Fatalf("expected embedded svg in html not to be detected as root svg")
	}
}

func TestDetectFormatRecognizesComplexOfficeFixtures(t *testing.T) {
	testCases := []struct {
		name         string
		fixture      string
		expectedMIME string
		expectedExt  string
	}{
		{
			name:         "complex pptx",
			fixture:      ingestionFixturePath("presentation", "valid-three-slides.pptx"),
			expectedMIME: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			expectedExt:  "pptx",
		},
		{
			name:         "multi-sheet xlsx",
			fixture:      ingestionFixturePath("spreadsheet", "valid-multi-sheet.xlsx"),
			expectedMIME: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			expectedExt:  "xlsx",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(tc.fixture)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			format, err := DetectFormat(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("DetectFormat returned error: %v", err)
			}
			if format.MIMEType != tc.expectedMIME {
				t.Fatalf("expected MIME %s, got %s", tc.expectedMIME, format.MIMEType)
			}
			if format.Extension != tc.expectedExt {
				t.Fatalf("expected extension %s, got %s", tc.expectedExt, format.Extension)
			}
			if format.Family != domain.FamilyDocument {
				t.Fatalf("expected document family, got %s", format.Family)
			}
		})
	}
}
