package document

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DOCXToMarkdownEngine converts DOCX to Markdown via LibreOffice HTML export.
type DOCXToMarkdownEngine struct{}

func (e *DOCXToMarkdownEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	htmlPath, err := (&ToHTMLEngine{}).Execute(ctx, inputPath, outputDir, "html")
	if err != nil {
		return "", err
	}

	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		return "", fmt.Errorf("read html: %w", err)
	}

	markdown, err := convertHTMLToMarkdown(string(htmlData))
	if err != nil {
		return "", fmt.Errorf("convert html to markdown: %w", err)
	}

	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return "", fmt.Errorf("converted markdown is empty")
	}

	outputPath := filepath.Join(outputDir, "converted.md")
	if err := os.WriteFile(outputPath, []byte(markdown+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write markdown: %w", err)
	}

	return outputPath, nil
}
