package pdf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/workers/document"
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

	// Sanitize the output to strip any remote references or scripts.
	outData, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read html output: %w", err)
	}
	if err := os.WriteFile(outputPath, document.SanitizeHTMLBytes(outData), 0o644); err != nil {
		return "", fmt.Errorf("write sanitized html output: %w", err)
	}

	return outputPath, nil
}
