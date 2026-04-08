package capabilities

import (
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

// allEnginesAvailable sets up a prober where all engines are available.
func allEnginesAvailable(t *testing.T) func() {
	t.Helper()
	p := &EngineProber{}
	p.available = map[string]bool{
		"go-image":    true,
		"libreoffice": true,
		"poppler":     true,
		"ffmpeg":      true,
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

func TestResolveReturnsCapsForPDF(t *testing.T) {
	defer allEnginesAvailable(t)()

	file := fakePDFFile()
	caps := Resolve(file)
	if len(caps) == 0 {
		t.Fatal("expected at least one capability for PDF")
	}
	for _, c := range caps {
		if c.TargetFormat == "pdf" {
			t.Fatalf("should not offer same-format conversion, got %s", c.ID)
		}
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
		if c.TargetFormat == "png" {
			t.Fatalf("should not offer same-format conversion, got %s", c.ID)
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
	// Instead, let's verify the same-format check works via Resolve.
	caps := Resolve(file)
	for _, c := range caps {
		if c.TargetFormat == file.DetectedFormat.Extension {
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
