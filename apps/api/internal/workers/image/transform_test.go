package image

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestJPEG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 80, B: 30, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create jpeg: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
}

func writeTestPNG(t *testing.T, path string, width, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 120, B: 220, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
}

func writeTestSVG(t *testing.T, path string) {
	t.Helper()
	source := `<svg xmlns="http://www.w3.org/2000/svg" width="180" height="120" viewBox="0 0 180 120"><rect width="180" height="120" fill="#0f766e"/><circle cx="90" cy="60" r="32" fill="#f8fafc"/></svg>`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write svg: %v", err)
	}
}

func writeLinkedTestSVG(t *testing.T, path string) {
	t.Helper()
	source := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="220" height="140" viewBox="0 0 220 140"><rect width="220" height="140" fill="#0f766e"/><a href="https://example.com" xlink:href="https://example.com"><text x="24" y="78" fill="#f8fafc" font-size="24">Reform Lab</text></a></svg>`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write svg with link: %v", err)
	}
}

func imageFixturePath(parts ...string) string {
	path := []string{"..", "..", "..", "tests", "fixtures"}
	path = append(path, parts...)
	return filepath.Join(path...)
}

func decodeImageSize(t *testing.T, path string) image.Config {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open image output: %v", err)
	}
	defer f.Close()
	decoded, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode image output: %v", err)
	}
	return decoded
}

func TestCompressEngineCreatesJPEG(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.jpg")
	writeTestJPEG(t, inputPath, 640, 480)

	engine := &CompressEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "jpg")
	if err != nil {
		t.Fatalf("compress jpg: %v", err)
	}
	if filepath.Ext(outputPath) != ".jpg" {
		t.Fatalf("expected jpg output, got %s", outputPath)
	}
	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("stat output: %v", err)
	}
}

func TestThumbnailEngineResizesPNG(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.png")
	writeTestPNG(t, inputPath, 1200, 800)

	engine := &ThumbnailEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "png")
	if err != nil {
		t.Fatalf("thumbnail png: %v", err)
	}

	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	decoded, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if decoded.Width > thumbnailMaxEdge || decoded.Height > thumbnailMaxEdge {
		t.Fatalf("expected thumbnail within %dpx, got %dx%d", thumbnailMaxEdge, decoded.Width, decoded.Height)
	}
}

func TestToWebPEngineCreatesWebP(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.png")
	writeTestPNG(t, inputPath, 128, 128)

	engine := &ToWebPEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "webp")
	if err != nil {
		t.Fatalf("image to webp: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.HasPrefix(string(data[:4]), "RIFF") {
		t.Fatalf("expected webp RIFF header, got %q", data[:4])
	}
}

func TestToAVIFEngineCreatesAVIF(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.png")
	writeTestPNG(t, inputPath, 128, 128)

	engine := &ToAVIFEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "avif")
	if err != nil {
		t.Fatalf("image to avif: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) < 12 || string(data[4:8]) != "ftyp" {
		t.Fatalf("expected ISO BMFF header, got %q", data[:min(len(data), 12)])
	}
	header := data[:min(len(data), 32)]
	if !bytes.Contains(header, []byte("avif")) && !bytes.Contains(header, []byte("avis")) {
		t.Fatalf("expected avif-compatible brand in header, got %q", header)
	}
}

func TestWebVariantEngineCreatesResizedJPG(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.png")
	writeTestPNG(t, inputPath, 2200, 1400)

	engine := NewWebVariantEngine(640)
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "jpg")
	if err != nil {
		t.Fatalf("image web jpg: %v", err)
	}

	f, err := os.Open(outputPath)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()
	decoded, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if decoded.Width > 640 || decoded.Height > 640 {
		t.Fatalf("expected resized JPG within 640px, got %dx%d", decoded.Width, decoded.Height)
	}
}

func TestWebVariantEngineCreatesWebP(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.jpg")
	writeTestJPEG(t, inputPath, 2200, 1400)

	engine := NewWebVariantEngine(1600)
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "webp")
	if err != nil {
		t.Fatalf("image web webp: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.HasPrefix(string(data[:4]), "RIFF") {
		t.Fatalf("expected webp RIFF header, got %q", data[:4])
	}
	if info, err := os.Stat(outputPath); err != nil || info.Size() == 0 {
		t.Fatalf("expected non-empty webp output, got err=%v size=%d", err, info.Size())
	}
}

func TestWebVariantEngineCreatesAVIF(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.jpg")
	writeTestJPEG(t, inputPath, 2201, 1401)

	engine := NewWebVariantEngine(1600)
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "avif")
	if err != nil {
		t.Fatalf("image web avif: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) < 12 || string(data[4:8]) != "ftyp" {
		t.Fatalf("expected ISO BMFF header, got %q", data[:min(len(data), 12)])
	}
	header := data[:min(len(data), 32)]
	if !bytes.Contains(header, []byte("avif")) && !bytes.Contains(header, []byte("avis")) {
		t.Fatalf("expected avif-compatible brand in header, got %q", header)
	}
	if info, err := os.Stat(outputPath); err != nil || info.Size() == 0 {
		t.Fatalf("expected non-empty avif output, got err=%v size=%d", err, info.Size())
	}
}

func TestSVGConvertEngineCreatesRasterOutputs(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.svg")
	writeTestSVG(t, inputPath)

	engine := &SVGConvertEngine{}
	for outputFormat, check := range map[string]func([]byte) bool{
		"png": func(data []byte) bool {
			return len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		},
		"webp": func(data []byte) bool { return len(data) >= 4 && strings.HasPrefix(string(data[:4]), "RIFF") },
	} {
		outputPath, err := engine.Execute(context.Background(), inputPath, dir, outputFormat)
		if err != nil {
			t.Fatalf("svg to %s: %v", outputFormat, err)
		}
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("read %s output: %v", outputFormat, err)
		}
		if !check(data) {
			t.Fatalf("unexpected %s output header: %q", outputFormat, data[:min(len(data), 12)])
		}
	}
}

func TestSVGConvertEngineCreatesVectorPDF(t *testing.T) {
	for _, bin := range []string{"rsvg-convert", "pdfinfo"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available", bin)
		}
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.svg")
	writeLinkedTestSVG(t, inputPath)

	outputPath, err := (&SVGConvertEngine{}).Execute(context.Background(), inputPath, dir, "pdf")
	if err != nil {
		t.Fatalf("svg to pdf: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read pdf output: %v", err)
	}
	if len(data) < 5 || !strings.HasPrefix(string(data[:5]), "%PDF-") {
		t.Fatalf("unexpected pdf output header: %q", data[:min(len(data), 12)])
	}
	infoOut, err := exec.Command("pdfinfo", "-url", outputPath).CombinedOutput()
	if err != nil {
		t.Fatalf("inspect pdf links: %v: %s", err, strings.TrimSpace(string(infoOut)))
	}
	if !strings.Contains(string(infoOut), "https://example.com") {
		t.Fatalf("expected vector pdf to preserve link annotation, got %q", strings.TrimSpace(string(infoOut)))
	}
}

func TestPrepareSanitizedSVGRemovesDangerousMarkup(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "input.svg")
	source := `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><foreignObject><body><script>alert(1)</script></body></foreignObject><use xlink:href="https://evil.example/icons.svg#shape"></use><image href="https://evil.example/image.png"></image><a href="https://example.com"><text x="10" y="20">link</text></a></svg>`
	if err := os.WriteFile(inputPath, []byte(source), 0o600); err != nil {
		t.Fatalf("write svg: %v", err)
	}

	sanitizedPath, cleanup, err := prepareSanitizedSVG(inputPath, dir)
	if err != nil {
		t.Fatalf("prepareSanitizedSVG: %v", err)
	}
	defer cleanup()

	sanitized, err := os.ReadFile(sanitizedPath)
	if err != nil {
		t.Fatalf("read sanitized svg: %v", err)
	}
	text := string(sanitized)
	if strings.Contains(strings.ToLower(text), "foreignobject") {
		t.Fatalf("expected foreignObject to be removed, got %s", text)
	}
	if strings.Contains(text, "https://evil.example") {
		t.Fatalf("expected remote resource URLs to be neutralized, got %s", text)
	}
	if !strings.Contains(text, `href="https://example.com"`) {
		t.Fatalf("expected navigational links to be preserved, got %s", text)
	}
}

func TestHEIFConvertEngineCreatesOutputs(t *testing.T) {
	for _, bin := range []string{"heif-convert", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available", bin)
		}
	}

	dir := t.TempDir()
	inputPath := imageFixturePath("heif", "valid-basic.heif")

	engine := &HEIFConvertEngine{}
	checks := map[string]func([]byte) bool{
		"jpg": func(data []byte) bool { return len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8 },
		"png": func(data []byte) bool {
			return len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
		},
		"webp": func(data []byte) bool { return len(data) >= 4 && strings.HasPrefix(string(data[:4]), "RIFF") },
	}
	for outputFormat, check := range checks {
		outputPath, err := engine.Execute(context.Background(), inputPath, dir, outputFormat)
		if err != nil {
			t.Fatalf("heif to %s: %v", outputFormat, err)
		}
		data, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatalf("read %s output: %v", outputFormat, err)
		}
		if !check(data) {
			t.Fatalf("unexpected %s output header: %q", outputFormat, data[:min(len(data), 12)])
		}
	}
}

func TestHEIFConvertEngineCreatesPNGFromComplexFixture(t *testing.T) {
	for _, bin := range []string{"heif-convert", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available", bin)
		}
	}

	dir := t.TempDir()
	inputPath := imageFixturePath("heif", "valid-complex.heif")

	outputPath, err := (&HEIFConvertEngine{}).Execute(context.Background(), inputPath, dir, "png")
	if err != nil {
		t.Fatalf("complex heif to png: %v", err)
	}
	decoded := decodeImageSize(t, outputPath)
	if decoded.Width != 960 || decoded.Height != 540 {
		t.Fatalf("expected complex heif to keep 960x540, got %dx%d", decoded.Width, decoded.Height)
	}
}

func TestHEIFConvertEngineRejectsCorruptedFixture(t *testing.T) {
	if _, err := exec.LookPath("heif-convert"); err != nil {
		t.Skip("heif-convert not available")
	}

	dir := t.TempDir()
	inputPath := imageFixturePath("heif", "corrupted-truncated.heif")

	if _, err := (&HEIFConvertEngine{}).Execute(context.Background(), inputPath, dir, "png"); err == nil {
		t.Fatal("expected corrupted heif fixture to fail conversion")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
