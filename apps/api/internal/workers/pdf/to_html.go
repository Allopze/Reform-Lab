package pdf

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToHTMLEngine converts PDF files to single-file HTML previews using pdftohtml.
type ToHTMLEngine struct{}

func (e *ToHTMLEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "preview.html")
	cmd := exec.CommandContext(ctx,
		"pdftohtml", "-s", "-noframes", "-enc", "UTF-8",
		inputPath, outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdftohtml: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}
