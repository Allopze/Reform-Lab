package pdf

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	workerImage "github.com/allopze/reform-lab/apps/api/internal/workers/image"
)

func TestCompressEngineCreatesCompressedPDF(t *testing.T) {
	if _, err := exec.LookPath("gs"); err != nil {
		t.Skip("ghostscript not available")
	}

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "source.png")
	img := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := 0; y < 300; y++ {
		for x := 0; x < 400; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 120, B: 40, A: 255})
		}
	}
	f, err := os.Create(imagePath)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		_ = f.Close()
		t.Fatalf("encode png: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close png: %v", err)
	}

	pdfPath, err := (&workerImage.ToPDFEngine{}).Execute(context.Background(), imagePath, dir, "pdf")
	if err != nil {
		t.Fatalf("prepare pdf input: %v", err)
	}

	outputPath, err := (&CompressEngine{}).Execute(context.Background(), pdfPath, dir, "pdf")
	if err != nil {
		t.Fatalf("compress pdf: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.HasPrefix(string(data[:5]), "%PDF-") {
		t.Fatalf("expected pdf header, got %q", data[:5])
	}
}
