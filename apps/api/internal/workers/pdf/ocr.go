package pdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/workers/ocrutil"
)

// OCRToTextEngine rasterizes a PDF and extracts OCR text from each page image.
type OCRToTextEngine struct{}

func (e *OCRToTextEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	pages, err := renderPDFPages(ctx, inputPath, outputDir, "ocr-page", "-png", "png")
	if err != nil {
		return "", err
	}

	textParts := make([]string, 0, len(pages))
	for index, pagePath := range pages {
		outputBase := ocrutil.OutputBase(outputDir, fmt.Sprintf("ocr-page-%02d", index+1))
		textPath, err := ocrutil.RunTesseract(ctx, pagePath, outputBase, "txt")
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(textPath)
		if err != nil {
			return "", fmt.Errorf("read OCR text: %w", err)
		}
		text := strings.TrimSpace(string(data))
		if text != "" {
			textParts = append(textParts, text)
		}
	}

	if len(textParts) == 0 {
		return "", fmt.Errorf("ocr produced no text")
	}

	outputPath := filepath.Join(outputDir, "ocr.txt")
	if err := os.WriteFile(outputPath, []byte(strings.Join(textParts, "\n\n")), 0o644); err != nil {
		return "", fmt.Errorf("write OCR text: %w", err)
	}
	return outputPath, nil
}

// OCRToJSONEngine produces page-structured OCR JSON derived from TSV output.
type OCRToJSONEngine struct{}

func (e *OCRToJSONEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	pages, err := renderPDFPages(ctx, inputPath, outputDir, "ocr-page", "-png", "png")
	if err != nil {
		return "", err
	}

	structuredPages := make([]ocrutil.Page, 0, len(pages))
	for index, pagePath := range pages {
		outputBase := ocrutil.OutputBase(outputDir, fmt.Sprintf("ocr-page-%02d", index+1))
		tsvPath, err := ocrutil.RunTesseract(ctx, pagePath, outputBase, "tsv")
		if err != nil {
			return "", err
		}
		page, err := ocrutil.ParseTSVFile(tsvPath, index+1)
		if err != nil {
			return "", err
		}
		structuredPages = append(structuredPages, page)
	}

	outputPath := filepath.Join(outputDir, "ocr.json")
	if err := ocrutil.WriteDocumentJSON(outputPath, structuredPages); err != nil {
		return "", err
	}
	return outputPath, nil
}

// OCRToSearchablePDFEngine rebuilds a searchable PDF by OCRing rasterized pages.
type OCRToSearchablePDFEngine struct{}

func (e *OCRToSearchablePDFEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	pages, err := renderPDFPages(ctx, inputPath, outputDir, "ocr-page", "-png", "png")
	if err != nil {
		return "", err
	}

	pagePDFs := make([]string, 0, len(pages))
	for index, pagePath := range pages {
		outputBase := ocrutil.OutputBase(outputDir, fmt.Sprintf("ocr-page-%02d", index+1))
		pdfPath, err := ocrutil.RunTesseract(ctx, pagePath, outputBase, "pdf")
		if err != nil {
			return "", err
		}
		pagePDFs = append(pagePDFs, pdfPath)
	}

	outputPath := filepath.Join(outputDir, "searchable.pdf")
	if err := ocrutil.MergePDFs(ctx, outputPath, pagePDFs); err != nil {
		return "", err
	}
	return outputPath, nil
}
