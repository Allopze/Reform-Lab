package document

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// ToPDFEngine converts LibreOffice-supported office inputs to PDF.
type ToPDFEngine struct{}

func (e *ToPDFEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	effectiveInput := inputPath
	if looksLikeHTML(inputPath) {
		safePath := filepath.Join(outputDir, "safe-input.html")
		if err := copyAndSanitizeHTML(inputPath, safePath); err != nil {
			return "", fmt.Errorf("sanitize html input: %w", err)
		}
		effectiveInput = safePath
	}

	if err := runLibreOfficeConvert(ctx, "libreoffice doc-to-pdf", "pdf", effectiveInput, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(effectiveInput), filepath.Ext(effectiveInput))
	outputPath := filepath.Join(outputDir, base+".pdf")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}
