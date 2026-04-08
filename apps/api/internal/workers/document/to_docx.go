package document

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToDocxEngine converts documents (ODT, RTF) to DOCX using LibreOffice.
type ToDocxEngine struct{}

func (e *ToDocxEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	cmd := exec.CommandContext(ctx,
		"libreoffice", "--headless", "--convert-to", "docx",
		"--outdir", outputDir, inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("libreoffice doc-to-docx: %s: %w", strings.TrimSpace(string(out)), err)
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	return filepath.Join(outputDir, base+".docx"), nil
}
