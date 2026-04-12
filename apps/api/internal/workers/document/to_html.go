package document

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ToHTMLEngine converts LibreOffice-supported office inputs to simple HTML.
type ToHTMLEngine struct{}

func (e *ToHTMLEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	// If the input is HTML, sanitize it before passing to LibreOffice.
	if looksLikeHTML(inputPath) {
		safePath := filepath.Join(outputDir, "safe-input.html")
		if err := copyAndSanitizeHTML(inputPath, safePath); err != nil {
			return "", fmt.Errorf("sanitize html input: %w", err)
		}
		inputPath = safePath
	}

	if err := runLibreOfficeConvert(ctx, "libreoffice doc-to-html", "html", inputPath, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, base+".html")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}

	// Sanitize the output HTML to strip any remote references or scripts.
	outData, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read html output: %w", err)
	}
	if err := os.WriteFile(outputPath, sanitizeHTMLBytes(outData), 0o644); err != nil {
		return "", fmt.Errorf("write sanitized html output: %w", err)
	}

	return outputPath, nil
}
