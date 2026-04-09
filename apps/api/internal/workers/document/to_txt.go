package document

import (
	"context"
	"path/filepath"
	"strings"
)

// ToTxtEngine extracts text from LibreOffice-supported documents.
type ToTxtEngine struct{}

func (e *ToTxtEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	if err := runLibreOfficeConvert(ctx, "libreoffice doc-to-txt", "txt:Text", inputPath, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, base+".txt")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}
