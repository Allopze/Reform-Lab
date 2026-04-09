package pdf

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CompressEngine reduces PDF size using Ghostscript's pdfwrite backend.
type CompressEngine struct{}

func (e *CompressEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted.pdf")
	args := []string{
		"-sDEVICE=pdfwrite",
		"-dCompatibilityLevel=1.4",
		"-dPDFSETTINGS=/ebook",
		"-dNOPAUSE",
		"-dQUIET",
		"-dBATCH",
		"-sOutputFile=" + outputPath,
		inputPath,
	}
	cmd := exec.CommandContext(ctx, "gs", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ghostscript pdf-compress: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return outputPath, nil
}
