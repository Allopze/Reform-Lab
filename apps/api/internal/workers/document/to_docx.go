package document

import (
	"context"
	"path/filepath"
	"strings"
)

// ToDocxEngine converts supported documents such as ODT, RTF, or HTML to DOCX using LibreOffice.
type ToDocxEngine struct{}

func (e *ToDocxEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	if err := runLibreOfficeConvert(ctx, "libreoffice doc-to-docx", "docx:Office Open XML Text", inputPath, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
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
