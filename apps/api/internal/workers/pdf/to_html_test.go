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

func TestToHTMLEngineCreatesHTMLPreview(t *testing.T) {
	if _, err := exec.LookPath("pdftohtml"); err != nil {
		t.Skip("pdftohtml not available")
	}

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "source.png")
	img := image.NewRGBA(image.Rect(0, 0, 320, 240))
	for y := 0; y < 240; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, color.RGBA{R: 80, G: 120, B: 200, A: 255})
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

	outputPath, err := (&ToHTMLEngine{}).Execute(context.Background(), pdfPath, dir, "html")
	if err != nil {
		t.Fatalf("pdf to html preview: %v", err)
	}
	htmlData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(htmlData)), "<html") {
		t.Fatalf("expected html preview output")
	}
}
