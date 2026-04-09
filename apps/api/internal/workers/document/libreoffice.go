package document

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func runLibreOfficeConvert(ctx context.Context, label, convertTo, inputPath, outputDir string) error {
	cmd := exec.CommandContext(ctx,
		"libreoffice", "--headless", "--convert-to", convertTo,
		"--outdir", outputDir, inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s: %w", label, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func ensureOutputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("expected output %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected output %s to be a file", path)
	}
	return nil
}
