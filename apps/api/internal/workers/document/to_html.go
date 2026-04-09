package document

import (
	"context"
	"path/filepath"
	"strings"
)

// ToHTMLEngine converts LibreOffice-supported office inputs to simple HTML.
type ToHTMLEngine struct{}

func (e *ToHTMLEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	if err := runLibreOfficeConvert(ctx, "libreoffice doc-to-html", "html", inputPath, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, base+".html")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}
