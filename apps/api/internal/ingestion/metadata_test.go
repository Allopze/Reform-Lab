package ingestion

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

func TestExtractSVGDimensionsReadsHeaderPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.svg")
	content := `<svg viewBox="0 0 640 480">` + strings.Repeat(" ", 5000) + `</svg>`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write svg: %v", err)
	}

	w, h, ok := extractSVGDimensions(path)
	if !ok {
		t.Fatal("expected SVG dimensions to be extracted")
	}
	if w != 640 || h != 480 {
		t.Fatalf("unexpected dimensions: %dx%d", w, h)
	}
}

func TestExtractMetadataDetectsEncryptedOOXML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protected.docx")
	writeZipFixture(t, path, map[string]string{
		"EncryptionInfo":   "metadata",
		"EncryptedPackage": "payload",
	})

	meta, err := ExtractMetadata(context.Background(), path, domain.DetectedFormat{
		Family:   domain.FamilyDocument,
		MIMEType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	})
	if err != nil {
		t.Fatalf("extract metadata: %v", err)
	}
	if !meta.IsProtected {
		t.Fatal("expected encrypted OOXML to be marked protected")
	}
}

func TestExtractMetadataDetectsEncryptedODF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protected.odt")
	writeZipFixture(t, path, map[string]string{
		"META-INF/manifest.xml": `<manifest:manifest xmlns:manifest="urn:oasis:names:tc:opendocument:xmlns:manifest:1.0"><manifest:file-entry manifest:full-path="content.xml"><manifest:encryption-data /></manifest:file-entry></manifest:manifest>`,
	})

	meta, err := ExtractMetadata(context.Background(), path, domain.DetectedFormat{
		Family:   domain.FamilyDocument,
		MIMEType: "application/vnd.oasis.opendocument.text",
	})
	if err != nil {
		t.Fatalf("extract metadata: %v", err)
	}
	if !meta.IsProtected {
		t.Fatal("expected encrypted ODF to be marked protected")
	}
}

func TestExtractMetadataDetectsProtectedFixtureFiles(t *testing.T) {
	testCases := []struct {
		name   string
		path   string
		format domain.DetectedFormat
	}{
		{
			name: "ooxml encrypted package markers",
			path: ingestionFixturePath("protected", "ooxml-encrypted-marker.docx"),
			format: domain.DetectedFormat{
				Family:   domain.FamilyDocument,
				MIMEType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			},
		},
		{
			name: "odf encrypted manifest marker",
			path: ingestionFixturePath("protected", "odf-encrypted-manifest.odt"),
			format: domain.DetectedFormat{
				Family:   domain.FamilyDocument,
				MIMEType: "application/vnd.oasis.opendocument.text",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := ExtractMetadata(context.Background(), tc.path, tc.format)
			if err != nil {
				t.Fatalf("extract metadata: %v", err)
			}
			if !meta.IsProtected {
				t.Fatal("expected protected fixture to be marked protected")
			}
		})
	}
}

func writeZipFixture(t *testing.T, path string, entries map[string]string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(file)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
}
