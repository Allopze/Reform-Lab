package document

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// MarkdownToDocxEngine renders Markdown to HTML and then converts it to DOCX.
type MarkdownToDocxEngine struct{}

func (e *MarkdownToDocxEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	source, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("read markdown: %w", err)
	}

	htmlDoc, err := renderMarkdownDocument(source)
	if err != nil {
		return "", err
	}
	htmlDoc = sanitizeHTMLBytes(htmlDoc)

	htmlPath := filepath.Join(outputDir, "rendered.html")
	if err := os.WriteFile(htmlPath, htmlDoc, 0o644); err != nil {
		return "", fmt.Errorf("write html: %w", err)
	}

	outputPath, err := (&ToDocxEngine{}).Execute(ctx, htmlPath, outputDir, "docx")
	if err != nil {
		return "", err
	}

	return outputPath, nil
}
