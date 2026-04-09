package image

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func TestOCRToTextEngineExtractsText(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "ocr.png")
	writeReadableTextPNG(t, inputPath, "HELLO OCR")

	outputPath, err := (&OCRToTextEngine{}).Execute(context.Background(), inputPath, dir, "txt")
	if err != nil {
		t.Fatalf("image ocr to txt: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read ocr text: %v", err)
	}
	text := strings.ToLower(string(data))
	if !strings.Contains(text, "hello") {
		t.Fatalf("expected OCR output to contain hello, got %q", text)
	}
}

func TestOCRToJSONEngineExtractsStructuredBlocks(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("tesseract not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "ocr.png")
	writeReadableTextPNG(t, inputPath, "HELLO OCR")

	outputPath, err := (&OCRToJSONEngine{}).Execute(context.Background(), inputPath, dir, "json")
	if err != nil {
		t.Fatalf("image ocr to json: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read ocr json: %v", err)
	}
	var payload struct {
		Text  string `json:"text"`
		Pages []struct {
			Blocks []struct {
				Lines []struct {
					Text string `json:"text"`
				} `json:"lines"`
			} `json:"blocks"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal ocr json: %v", err)
	}
	if len(payload.Pages) == 0 || len(payload.Pages[0].Blocks) == 0 {
		t.Fatalf("expected at least one OCR block, got %s", data)
	}
	if !strings.Contains(strings.ToLower(payload.Text), "hello") {
		t.Fatalf("expected OCR text to contain hello, got %q", payload.Text)
	}
}

func writeReadableTextPNG(t *testing.T, path, text string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 900, 240))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

	parsedFont, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse font: %v", err)
	}
	face, err := opentype.NewFace(parsedFont, &opentype.FaceOptions{Size: 54, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		t.Fatalf("build face: %v", err)
	}

	drawer := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: face,
		Dot:  fixed.P(48, 140),
	}
	drawer.DrawString(text)

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode image: %v", err)
	}
}
