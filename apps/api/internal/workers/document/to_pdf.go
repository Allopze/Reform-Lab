package document

import (
	"context"
	"path/filepath"
	"strings"
)

// ToPDFEngine converts LibreOffice-supported office inputs to PDF.
type ToPDFEngine struct{}

func (e *ToPDFEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	if err := runLibreOfficeConvert(ctx, "libreoffice doc-to-pdf", "pdf", inputPath, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, base+".pdf")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}
