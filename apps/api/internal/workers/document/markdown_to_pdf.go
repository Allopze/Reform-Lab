package document

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// MarkdownToPDFEngine renders Markdown to HTML and then converts it to PDF.
type MarkdownToPDFEngine struct{}

func (e *MarkdownToPDFEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	source, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("read markdown: %w", err)
	}

	htmlDoc, err := renderMarkdownDocument(source)
	if err != nil {
		return "", err
	}

	htmlPath := filepath.Join(outputDir, "rendered.html")
	if err := os.WriteFile(htmlPath, htmlDoc, 0o644); err != nil {
		return "", fmt.Errorf("write html: %w", err)
	}

	if err := runLibreOfficeConvert(ctx, "libreoffice markdown-to-pdf", "pdf", htmlPath, outputDir); err != nil {
		return "", err
	}

	outputPath := filepath.Join(outputDir, "rendered.pdf")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}
