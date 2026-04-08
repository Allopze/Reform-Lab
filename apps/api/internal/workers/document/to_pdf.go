package document

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToPDFEngine converts documents (DOCX, ODT, RTF) to PDF using LibreOffice.
type ToPDFEngine struct{}

func (e *ToPDFEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	cmd := exec.CommandContext(ctx,
		"libreoffice", "--headless", "--convert-to", "pdf",
		"--outdir", outputDir, inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("libreoffice doc-to-pdf: %s: %w", strings.TrimSpace(string(out)), err)
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	return filepath.Join(outputDir, base+".pdf"), nil
}
