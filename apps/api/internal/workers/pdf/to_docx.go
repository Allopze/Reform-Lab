package pdf

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToDocxEngine converts PDF to DOCX using the dedicated pdf2docx CLI.
type ToDocxEngine struct{}

func (e *ToDocxEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted.docx")
	cmd := exec.CommandContext(ctx, "pdf2docx", "convert", inputPath, outputPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("pdf2docx pdf-to-docx: %s: %w", strings.TrimSpace(string(out)), err)
	}
	if err := ensureOutputFile(outputPath); err != nil {
		trimmedOutput := strings.TrimSpace(string(out))
		if trimmedOutput == "" {
			return "", fmt.Errorf("pdf2docx pdf-to-docx: %w", err)
		}
		return "", fmt.Errorf("pdf2docx pdf-to-docx: %s: %w", trimmedOutput, err)
	}
	return outputPath, nil
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
