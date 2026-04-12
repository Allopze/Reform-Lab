package ingestion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
