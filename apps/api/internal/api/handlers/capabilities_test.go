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
