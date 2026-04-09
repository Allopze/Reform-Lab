package image

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SVGConvertEngine rasterizes bitmap outputs and keeps vector PDF export via librsvg.
type SVGConvertEngine struct{}

func (e *SVGConvertEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	switch outputFormat {
	case "png":
		outputPath := filepath.Join(outputDir, "converted.png")
		if err := renderSVG(ctx, inputPath, outputPath); err != nil {
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
		if err := renderSVG(ctx, inputPath, rasterPath); err != nil {
			return "", err
		}
		return (&ToWebPEngine{}).Execute(ctx, rasterPath, outputDir, outputFormat)
	case "pdf":
		outputPath := filepath.Join(outputDir, "converted.pdf")
		if err := renderSVGToPDF(ctx, inputPath, outputPath); err != nil {
			return "", err
		}
		return outputPath, nil
	default:
		return "", fmt.Errorf("unsupported svg output format: %s", outputFormat)
	}
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
