package document

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToTxtEngine extracts text from documents using LibreOffice.
type ToTxtEngine struct{}

func (e *ToTxtEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	cmd := exec.CommandContext(ctx,
		"libreoffice", "--headless", "--convert-to", "txt:Text",
		"--outdir", outputDir, inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("libreoffice doc-to-txt: %s: %w", strings.TrimSpace(string(out)), err)
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	return filepath.Join(outputDir, base+".txt"), nil
}
