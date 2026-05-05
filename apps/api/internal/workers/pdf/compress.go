package pdf

import (
	"context"
	"fmt"
	"os"
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
	// Ghostscript requires a POSIX-compatible temp directory. The VS Code remote
	// temp directory (e.g. ~/.vscode-server-insiders/tmp/...) is not compatible
	// with Ghostscript's temp file handling, so force TMPDIR to /tmp.
	env := os.Environ()
	for i, e := range env {
		if strings.HasPrefix(e, "TMPDIR=") {
			env[i] = "TMPDIR=/tmp"
			cmd.Env = env
			break
		}
	}
	if cmd.Env == nil {
		cmd.Env = append(env, "TMPDIR=/tmp")
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ghostscript pdf-compress: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return outputPath, nil
}
