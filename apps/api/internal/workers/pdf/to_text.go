package pdf

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToTextEngine extracts text from a PDF using Poppler's pdftotext.
type ToTextEngine struct{}

func (e *ToTextEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "extracted.txt")

	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", inputPath, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdftotext: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}
