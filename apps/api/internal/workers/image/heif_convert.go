package image

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// HEIFConvertEngine decodes HEIC/HEIF images through libheif tooling.
type HEIFConvertEngine struct{}

func (e *HEIFConvertEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	switch outputFormat {
	case "jpg", "png":
		outputPath := filepath.Join(outputDir, "converted."+outputFormat)
		return decodeHEIF(ctx, inputPath, outputPath)
	case "webp":
		tmpDir, err := os.MkdirTemp(outputDir, "heif-webp-")
		if err != nil {
			return "", fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		rasterPath, err := decodeHEIF(ctx, inputPath, filepath.Join(tmpDir, "decoded.png"))
		if err != nil {
			return "", err
		}
		return (&ToWebPEngine{}).Execute(ctx, rasterPath, outputDir, outputFormat)
	default:
		return "", fmt.Errorf("unsupported heif output format: %s", outputFormat)
	}
}

func decodeHEIF(ctx context.Context, inputPath, outputPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "heif-convert", inputPath, outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("heif-convert: %s: %w", strings.TrimSpace(string(out)), err)
	}

	if _, err := os.Stat(outputPath); err == nil {
		return outputPath, nil
	}

	ext := strings.TrimPrefix(filepath.Ext(outputPath), ".")
	stem := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
	matches, err := filepath.Glob(stem + "-*" + "." + ext)
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("heif-convert produced no output files")
	}
	sort.Strings(matches)
	return matches[0], nil
}
