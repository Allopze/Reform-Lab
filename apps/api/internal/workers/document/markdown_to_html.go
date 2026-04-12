package document

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// MarkdownToHTMLEngine renders Markdown files to standalone HTML.
type MarkdownToHTMLEngine struct{}

func (e *MarkdownToHTMLEngine) Execute(_ context.Context, inputPath, outputDir, _ string) (string, error) {
	source, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("read markdown: %w", err)
	}

	htmlDoc, err := renderMarkdownDocument(source)
	if err != nil {
		return "", err
	}

	// Neutralize remote references and scripts in the generated HTML.
	htmlDoc = sanitizeHTMLBytes(htmlDoc)

	outputPath := filepath.Join(outputDir, "converted.html")
	if err := os.WriteFile(outputPath, htmlDoc, 0o644); err != nil {
		return "", fmt.Errorf("write html: %w", err)
	}
	return outputPath, nil
}
