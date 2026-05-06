package capabilities

import (
	"os/exec"
	"sync"
)

// engineBinaries maps engine names (as declared in Capability.Engine) to the
// binary that must be available on $PATH for the engine to work.
var engineBinaries = map[string][]string{
	"libreoffice":         {"libreoffice"},
	"pdf2docx":            {"pdf2docx"},
	"poppler":             {"pdftoppm", "pdftotext"},
	"poppler-html":        {"pdftohtml"},
	"ocr-pdf":             {"pdftoppm", "tesseract", "gs"},
	"ffmpeg":              {"ffmpeg"},
	"ghostscript":         {"gs"},
	"libheif":             {"heif-convert", "ffmpeg"},
	"librsvg":             {"rsvg-convert"},
	"libreoffice-poppler": {"libreoffice", "pdftoppm"},
	"goldmark":            {},
	"go-html":             {},
	"go-image":            {}, // pure Go — always available
	"tesseract":           {"tesseract"},
}

// EngineProber caches which engines are available at process startup.
type EngineProber struct {
	available map[string]bool
	declared  bool
	once      sync.Once
}

// DefaultProber is the package-level prober used by Resolve/IsEligible.
var DefaultProber = &EngineProber{}

// NewDeclaredEngineProber returns a prober that declares every known engine
// available without probing the runtime binaries. This is useful for the API
// process in production, where conversion happens in the worker container.
func NewDeclaredEngineProber() *EngineProber {
	available := make(map[string]bool)
	for engine := range engineBinaries {
		available[engine] = true
	}
	for _, capability := range Catalog {
		if capability.Engine == "" {
			continue
		}
		available[capability.Engine] = true
	}
	return &EngineProber{available: available, declared: true}
}

// Probe checks engine availability once and caches the result.
func (p *EngineProber) Probe() {
	p.once.Do(func() {
		if p.declared {
			if p.available == nil {
				p.available = make(map[string]bool)
			}
			return
		}
		p.available = make(map[string]bool)
		for engine, bins := range engineBinaries {
			ok := true
			for _, bin := range bins {
				if _, err := exec.LookPath(bin); err != nil {
					ok = false
					break
				}
			}
			p.available[engine] = ok
		}
	})
}

// IsAvailable returns true if the named engine was found during probing.
// Unknown engines are treated as unavailable.
func (p *EngineProber) IsAvailable(engine string) bool {
	p.Probe()
	return p.available[engine]
}

// AvailableEngines returns a snapshot of all probed engine statuses.
func (p *EngineProber) AvailableEngines() map[string]bool {
	p.Probe()
	result := make(map[string]bool, len(p.available))
	for k, v := range p.available {
		result[k] = v
	}
	return result
}
