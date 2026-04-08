package capabilities

import (
	"testing"
)

func TestEngineProberAlwaysAvailableGoImage(t *testing.T) {
	p := &EngineProber{}
	p.Probe()
	if !p.IsAvailable("go-image") {
		t.Fatal("go-image engine should always be available")
	}
}

func TestEngineProberUnknownEngineUnavailable(t *testing.T) {
	p := &EngineProber{}
	p.Probe()
	if p.IsAvailable("nonexistent-engine-xyz") {
		t.Fatal("unknown engine should not be available")
	}
}

func TestEngineProberAvailableEngines(t *testing.T) {
	p := &EngineProber{}
	result := p.AvailableEngines()
	if _, ok := result["go-image"]; !ok {
		t.Fatal("AvailableEngines should include go-image")
	}
	// Mutating the returned map should not affect internal state.
	result["go-image"] = false
	if !p.IsAvailable("go-image") {
		t.Fatal("mutating returned map should not affect prober state")
	}
}

func TestResolveFiltersUnavailableEngines(t *testing.T) {
	// Create a prober with all engines marked unavailable except go-image.
	p := &EngineProber{}
	p.available = map[string]bool{
		"go-image":    true,
		"libreoffice": false,
		"poppler":     false,
		"ffmpeg":      false,
	}
	p.once.Do(func() {}) // prevent re-probing

	old := DefaultProber
	DefaultProber = p
	defer func() { DefaultProber = old }()

	// Build a fake file that matches PDF capabilities.
	file := fakePDFFile()
	caps := Resolve(file)
	for _, c := range caps {
		if c.Engine != "go-image" {
			t.Fatalf("expected only go-image capabilities, got engine %q (cap %s)", c.Engine, c.ID)
		}
	}
}

func TestEffectiveEngineAvailabilityAppliesFeatureFlags(t *testing.T) {
	p := &EngineProber{}
	p.available = map[string]bool{
		"go-image":    true,
		"libreoffice": true,
		"poppler":     true,
		"ffmpeg":      true,
	}
	p.once.Do(func() {})

	oldProber := DefaultProber
	oldFlags := DefaultFlags
	DefaultProber = p
	DefaultFlags = NewFeatureFlags(nil, []string{"poppler"})
	defer func() {
		DefaultProber = oldProber
		DefaultFlags = oldFlags
	}()

	engines := EffectiveEngineAvailability()
	if engines["poppler"] {
		t.Fatal("expected poppler to be reported unavailable when disabled by feature flag")
	}
	if !engines["go-image"] {
		t.Fatal("expected go-image to remain available")
	}
}
