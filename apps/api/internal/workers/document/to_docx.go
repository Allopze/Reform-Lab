package document

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// ToDocxEngine converts supported documents such as ODT, RTF, or HTML to DOCX using LibreOffice.
type ToDocxEngine struct{}

func (e *ToDocxEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	effectiveInput := inputPath
	if looksLikeHTML(inputPath) {
		safePath := filepath.Join(outputDir, "safe-input.html")
		if err := copyAndSanitizeHTML(inputPath, safePath); err != nil {
			return "", fmt.Errorf("sanitize html input: %w", err)
		}
		effectiveInput = safePath
	}

	if err := runLibreOfficeConvert(ctx, "libreoffice doc-to-docx", "docx:Office Open XML Text", effectiveInput, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(effectiveInput), filepath.Ext(effectiveInput))
	outputPath := filepath.Join(outputDir, base+".docx")
	if err := ensureOutputFile(outputPath); err == nil {
		return outputPath, nil
	}

	matches, err := filepath.Glob(filepath.Join(outputDir, base+"*.docx"))
	if err == nil && len(matches) > 0 {
		if err := ensureOutputFile(matches[0]); err != nil {
			return "", err
		}
		return matches[0], nil
	}

	return "", ensureOutputFile(outputPath)
}
