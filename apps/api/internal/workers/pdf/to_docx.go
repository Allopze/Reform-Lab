package pdf

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToDocxEngine converts PDF to DOCX using LibreOffice headless.
type ToDocxEngine struct{}

func (e *ToDocxEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	cmd := exec.CommandContext(ctx,
		"libreoffice", "--headless", "--convert-to", "docx",
		"--outdir", outputDir, inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("libreoffice pdf-to-docx: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// LibreOffice names the output based on the input filename.
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, base+".docx")
	return outputPath, nil
}
