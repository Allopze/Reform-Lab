package document

import (
	"context"
	"path/filepath"
	"strings"
)

// ToCSVEngine exports spreadsheet-compatible inputs to CSV using LibreOffice.
type ToCSVEngine struct{}

func (e *ToCSVEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	if err := runLibreOfficeConvert(ctx, "libreoffice spreadsheet-to-csv", "csv", inputPath, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, base+".csv")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}
