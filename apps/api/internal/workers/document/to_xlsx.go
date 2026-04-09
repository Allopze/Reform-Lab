package document

import (
	"context"
	"path/filepath"
	"strings"
)

// ToXLSXEngine exports spreadsheet-compatible inputs to XLSX using LibreOffice.
type ToXLSXEngine struct{}

func (e *ToXLSXEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	if err := runLibreOfficeConvert(ctx, "libreoffice spreadsheet-to-xlsx", "xlsx", inputPath, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, base+".xlsx")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}
