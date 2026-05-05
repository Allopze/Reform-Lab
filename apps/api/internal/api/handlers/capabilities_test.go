package handlers

import (
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

func TestIntersectCapabilitiesKeepsStableCommonOrder(t *testing.T) {
	withoutExternalEngines(t)

	common := intersectCapabilities([]*domain.OriginalFile{
		testFile("photo.png", "image/png", domain.FamilyImage, "png"),
		testFile("photo.jpg", "image/jpeg", domain.FamilyImage, "jpg"),
	})

	if len(common) != 1 {
		t.Fatalf("expected one common capability, got %d: %#v", len(common), common)
	}
	if common[0].ID != "image-to-pdf" {
		t.Fatalf("expected image-to-pdf as the only common capability, got %q", common[0].ID)
	}
}

func TestIntersectCapabilitiesReturnsEmptyForMixedFamilies(t *testing.T) {
	withoutExternalEngines(t)

	common := intersectCapabilities([]*domain.OriginalFile{
		testFile("document.pdf", "application/pdf", domain.FamilyPDF, "pdf"),
		testFile("photo.jpg", "image/jpeg", domain.FamilyImage, "jpg"),
	})

	if len(common) != 0 {
		t.Fatalf("expected no common capabilities for mixed families, got %#v", common)
	}
}

func TestCatalogByFamilyIncludesLegacyDocCapability(t *testing.T) {
	grouped := catalogByFamily(capabilities.Catalog)

	var documentCaps []catalogCapabilityResponse
	for _, family := range grouped {
		if family.Family == string(domain.FamilyDocument) {
			documentCaps = family.Capabilities
			break
		}
	}
	if len(documentCaps) == 0 {
		t.Fatal("expected document capabilities in catalog")
	}

	for _, cap := range documentCaps {
		if cap.ID != "doc-to-docx" {
			continue
		}
		for _, source := range cap.SourceFormats {
			if source == "application/msword" {
				return
			}
		}
		t.Fatal("expected doc-to-docx to expose application/msword source format")
	}
	t.Fatal("expected doc-to-docx in catalog")
}

func TestCatalogByFamilyPreservesCatalogOrder(t *testing.T) {
	grouped := catalogByFamily(capabilities.Catalog)
	if len(grouped) == 0 || len(grouped[0].Capabilities) == 0 {
		t.Fatal("expected non-empty catalog")
	}
	if grouped[0].Capabilities[0].ID != capabilities.Catalog[0].ID {
		t.Fatalf("expected first catalog capability %q, got %q", capabilities.Catalog[0].ID, grouped[0].Capabilities[0].ID)
	}
}

func withoutExternalEngines(t *testing.T) {
	t.Helper()

	oldProber := capabilities.DefaultProber
	capabilities.DefaultProber = &capabilities.EngineProber{}
	t.Setenv("PATH", "")
	t.Cleanup(func() {
		capabilities.DefaultProber = oldProber
	})
}

func testFile(
	name string,
	mimeType string,
	family domain.FormatFamily,
	extension string,
) *domain.OriginalFile {
	return &domain.OriginalFile{
		ID:           uuid.New(),
		InternalName: name,
		OriginalName: name,
		Size:         1024,
		DetectedFormat: domain.DetectedFormat{
			MIMEType:  mimeType,
			Family:    family,
			Extension: extension,
		},
		UploadedAt: time.Now(),
	}
}
