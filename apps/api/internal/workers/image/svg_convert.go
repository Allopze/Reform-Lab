package image

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	workerDocument "github.com/allopze/reform-lab/apps/api/internal/workers/document"
)

// SVGConvertEngine rasterizes bitmap outputs and keeps vector PDF export via librsvg.
type SVGConvertEngine struct{}

func (e *SVGConvertEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	sanitizedInputPath, cleanup, err := prepareSanitizedSVG(inputPath, outputDir)
	if err != nil {
		return "", err
	}
	defer cleanup()

	switch outputFormat {
	case "png":
		outputPath := filepath.Join(outputDir, "converted.png")
		if err := renderSVG(ctx, sanitizedInputPath, outputPath); err != nil {
			return "", err
		}
		return outputPath, nil
	case "webp":
		tmpDir, err := os.MkdirTemp(outputDir, "svg-webp-")
		if err != nil {
			return "", fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		rasterPath := filepath.Join(tmpDir, "rendered.png")
		if err := renderSVG(ctx, sanitizedInputPath, rasterPath); err != nil {
			return "", err
		}
		return (&ToWebPEngine{}).Execute(ctx, rasterPath, outputDir, outputFormat)
	case "pdf":
		outputPath := filepath.Join(outputDir, "converted.pdf")
		if err := renderSVGToPDF(ctx, sanitizedInputPath, outputPath); err != nil {
			return "", err
		}
		return outputPath, nil
	default:
		return "", fmt.Errorf("unsupported svg output format: %s", outputFormat)
	}
}

func prepareSanitizedSVG(inputPath, outputDir string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp(outputDir, "svg-sanitized-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create sanitized svg temp dir: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	content, err := os.ReadFile(inputPath)
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("read svg: %w", err)
	}

	sanitizedPath := filepath.Join(tmpDir, "input.svg")
	sanitizedContent := workerDocument.SanitizeHTMLBytes(content)
	sanitizedContent = extractSVGDocument(sanitizedContent)
	if len(sanitizedContent) == 0 {
		cleanup()
		return "", func() {}, fmt.Errorf("sanitize svg: missing svg root after sanitization")
	}
	if err := os.WriteFile(sanitizedPath, sanitizedContent, 0o600); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write sanitized svg: %w", err)
	}

	return sanitizedPath, cleanup, nil
}

func extractSVGDocument(content []byte) []byte {
	text := string(content)
	lower := strings.ToLower(text)
	start := strings.Index(lower, "<svg")
	end := strings.LastIndex(lower, "</svg>")
	if start == -1 || end == -1 {
		return nil
	}
	end += len("</svg>")
	return []byte(text[start:end])
}

func renderSVG(ctx context.Context, inputPath, outputPath string) error {
	cmd := exec.CommandContext(ctx,
		"ffmpeg", "-y",
		"-i", inputPath,
		"-frames:v", "1",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg svg render: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func renderSVGToPDF(ctx context.Context, inputPath, outputPath string) error {
	cmd := exec.CommandContext(ctx,
		"rsvg-convert",
		"--format=pdf",
		"--output", outputPath,
		inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rsvg-convert svg to pdf: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
