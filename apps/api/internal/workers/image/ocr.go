package image

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/workers/ocrutil"
)

// OCRToTextEngine extracts OCR text from raster images via Tesseract.
type OCRToTextEngine struct{}

func (e *OCRToTextEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	normalizedPath, err := normalizeForOCR(ctx, inputPath, outputDir)
	if err != nil {
		return "", err
	}

	textPath, err := ocrutil.RunTesseract(ctx, normalizedPath, ocrutil.OutputBase(outputDir, "ocr-image"), "txt")
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(textPath)
	if err != nil {
		return "", fmt.Errorf("read OCR text: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", fmt.Errorf("ocr produced no text")
	}
	return textPath, nil
}

// OCRToJSONEngine extracts structured OCR blocks, lines, and words from images.
type OCRToJSONEngine struct{}

func (e *OCRToJSONEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	normalizedPath, err := normalizeForOCR(ctx, inputPath, outputDir)
	if err != nil {
		return "", err
	}

	tsvPath, err := ocrutil.RunTesseract(ctx, normalizedPath, ocrutil.OutputBase(outputDir, "ocr-image"), "tsv")
	if err != nil {
		return "", err
	}
	page, err := ocrutil.ParseTSVFile(tsvPath, 1)
	if err != nil {
		return "", err
	}

	outputPath := filepath.Join(outputDir, "ocr.json")
	if err := ocrutil.WriteDocumentJSON(outputPath, []ocrutil.Page{page}); err != nil {
		return "", err
	}
	return outputPath, nil
}

func normalizeForOCR(ctx context.Context, inputPath, outputDir string) (string, error) {
	path, err := (&ConvertEngine{}).Execute(ctx, inputPath, outputDir, "png")
	if err != nil {
		return "", fmt.Errorf("normalize image for OCR: %w", err)
	}
	return path, nil
}
