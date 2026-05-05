package ingestion

import (
	"archive/zip"
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

func TestDetectFormatRecognizesLegacyDocFixture(t *testing.T) {
	data, err := os.ReadFile(ingestionFixturePath("doc", "valid-basic.doc"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	format, err := DetectFormat(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DetectFormat returned error: %v", err)
	}
	if format.MIMEType != "application/msword" {
		t.Fatalf("expected application/msword, got %s", format.MIMEType)
	}
	if format.Extension != "doc" {
		t.Fatalf("expected doc extension, got %s", format.Extension)
	}
	if format.Family != domain.FamilyDocument {
		t.Fatalf("expected document family, got %s", format.Family)
	}
}

func TestDetectOOXMLMimeRejectsExtremeZipCompressionRatio(t *testing.T) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	entry, err := writer.Create("word/document.xml")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write(bytes.Repeat([]byte("0"), 256*1024)); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}

	if got := detectOOXMLMimeFromZip(bytes.NewReader(buf.Bytes()), int64(buf.Len())); got != "" {
		t.Fatalf("expected suspicious ZIP to be rejected, got %s", got)
	}
}

func TestDetectFormatRejectsControlledZipBombFixture(t *testing.T) {
	data, err := os.ReadFile(ingestionFixturePath("security", "zip-bomb-controlled.docx"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	if format, err := DetectFormat(bytes.NewReader(data)); err == nil {
		t.Fatalf("expected controlled zip bomb fixture to be unsupported, got format %+v", format)
	}
}

func TestCorruptedFixturesExerciseDetectionBoundaries(t *testing.T) {
	testCases := []struct {
		name         string
		fixtureParts []string
		expectedMIME string
		wantErr      bool
	}{
		{
			name:         "truncated pdf keeps pdf boundary",
			fixtureParts: []string{"pdf", "corrupted-truncated.pdf"},
			expectedMIME: "application/pdf",
		},
		{
			name:         "truncated png keeps image boundary",
			fixtureParts: []string{"image", "corrupted-truncated.png"},
			expectedMIME: "image/png",
		},
		{
			name:         "truncated wav keeps audio boundary",
			fixtureParts: []string{"audio", "corrupted-truncated.wav"},
			expectedMIME: "audio/wav",
		},
		{
			name:         "truncated mp4 keeps video boundary",
			fixtureParts: []string{"video", "corrupted-truncated.mp4"},
			expectedMIME: "video/mp4",
		},
		{
			name:         "broken docx zip is unsupported",
			fixtureParts: []string{"docx", "corrupted-broken-zip.docx"},
			wantErr:      true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := os.ReadFile(ingestionFixturePath(tc.fixtureParts...))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			format, err := DetectFormat(bytes.NewReader(data))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected detection error, got format %+v", format)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectFormat returned error: %v", err)
			}
			if format.MIMEType != tc.expectedMIME {
				t.Fatalf("expected MIME %s, got %s", tc.expectedMIME, format.MIMEType)
			}
		})
	}
}

func TestDetectFormatMarkdownFalsePositives(t *testing.T) {
	// Plain text with a single # should NOT be detected as markdown
	// (needs at least 2 strong signals or 1 strong + 1 weak)
	falsePositives := []struct {
		name  string
		input []byte
	}{
		{
			name:  "single hash in text",
			input: []byte("# This is just a comment in a config file\n"),
		},
		{
			name:  "hash in code comment",
			input: []byte("# This is a comment\n# Another comment\nSome text\n"),
		},
		{
			name:  "plain text with numbers",
			input: []byte("1. First item\n2. Second item\n3. Third item\n"),
		},
		{
			name:  "text with dashes but no markdown structure",
			input: []byte("item-one\nitem-two\nitem-three\n"),
		},
		{
			name:  "text with pipe characters but no table",
			input: []byte("name|age|city\n"),
		},
		{
			name:  "text with blockquote-like line",
			input: []byte("> This is a quote\n"),
		},
		{
			name:  "text with horizontal rule-like line",
			input: []byte("---\n"),
		},
		{
			name:  "empty file",
			input: []byte(""),
		},
		{
			name:  "whitespace only",
			input: []byte("   \n\n   \n"),
		},
	}

	for _, tc := range falsePositives {
		t.Run(tc.name, func(t *testing.T) {
			format, err := DetectFormat(bytes.NewReader(tc.input))
			if err != nil {
				t.Fatalf("DetectFormat returned error: %v", err)
			}
			if format.MIMEType == "text/markdown" {
				t.Fatalf("should NOT detect as markdown, got %s for input: %q", format.MIMEType, string(tc.input))
			}
		})
	}
}

func TestDetectFormatMarkdownTruePositives(t *testing.T) {
	// These should be detected as markdown
	truePositives := []struct {
		name  string
		input []byte
	}{
		{
			name:  "heading and list",
			input: []byte("# Title\n\n- item one\n- item two\n"),
		},
		{
			name:  "heading and link",
			input: []byte("# Title\n\n[link](https://example.com)\n"),
		},
		{
			name:  "code fence and heading",
			input: []byte("# Title\n\n```python\nprint('hello')\n```\n"),
		},
		{
			name:  "heading and table",
			input: []byte("# Title\n\n| col1 | col2 |\n|------|------|\n"),
		},
		{
			name:  "heading and blockquote",
			input: []byte("# Title\n\n> This is a quote\n"),
		},
		{
			name:  "heading and horizontal rule",
			input: []byte("# Title\n\n---\n"),
		},
		{
			name:  "multiple headings and list",
			input: []byte("# Title\n\n## Section\n\n- item one\n"),
		},
	}

	for _, tc := range truePositives {
		t.Run(tc.name, func(t *testing.T) {
			format, err := DetectFormat(bytes.NewReader(tc.input))
			if err != nil {
				t.Fatalf("DetectFormat returned error: %v", err)
			}
			if format.MIMEType != "text/markdown" {
				t.Fatalf("expected text/markdown, got %s for input: %q", format.MIMEType, string(tc.input))
			}
		})
	}
}

func TestDetectFormatSVGVariants(t *testing.T) {
	// SVG with XML namespace should be detected as image/svg+xml
	svgWithNamespace := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100">
  <circle cx="50" cy="50" r="40"/>
</svg>`)
	format, err := DetectFormat(bytes.NewReader(svgWithNamespace))
	if err != nil {
		t.Fatalf("DetectFormat returned error: %v", err)
	}
	if format.MIMEType != "image/svg+xml" {
		t.Fatalf("expected image/svg+xml, got %s", format.MIMEType)
	}

	// SVG without XML declaration but with namespace
	svgNoXMLDecl := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <rect width="100" height="100" fill="red"/>
</svg>`)
	format, err = DetectFormat(bytes.NewReader(svgNoXMLDecl))
	if err != nil {
		t.Fatalf("DetectFormat returned error: %v", err)
	}
	if format.MIMEType != "image/svg+xml" {
		t.Fatalf("expected image/svg+xml, got %s", format.MIMEType)
	}

	// Small inline SVG should still be detected
	smallSVG := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg>`)
	format, err = DetectFormat(bytes.NewReader(smallSVG))
	if err != nil {
		t.Fatalf("DetectFormat returned error: %v", err)
	}
	if format.MIMEType != "image/svg+xml" {
		t.Fatalf("expected image/svg+xml, got %s", format.MIMEType)
	}
}

func TestDetectFormatSVGFalsePositives(t *testing.T) {
	// Text that contains "svg" but is not SVG should not be detected as SVG
	falsePositives := []struct {
		name        string
		input       []byte
		expectError bool
	}{
		{
			name:  "text mentioning svg",
			input: []byte("This file contains an SVG image.\n"),
		},
		{
			name:  "html with svg-like content",
			input: []byte(`<div class="svg-container">Some content</div>`),
		},
		{
			name:        "xml that is not svg",
			input:       []byte(`<?xml version="1.0"?><root><item>test</item></root>`),
			expectError: true,
		},
	}

	for _, tc := range falsePositives {
		t.Run(tc.name, func(t *testing.T) {
			format, err := DetectFormat(bytes.NewReader(tc.input))
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for non-SVG XML, got format %s", format.MIMEType)
				}
				return
			}
			if err != nil {
				t.Fatalf("DetectFormat returned error: %v", err)
			}
			if format.MIMEType == "image/svg+xml" {
				t.Fatalf("should NOT detect as SVG, got %s for input: %q", format.MIMEType, string(tc.input))
			}
		})
	}
}
