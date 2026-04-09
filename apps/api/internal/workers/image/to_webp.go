package image

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToWebPEngine converts raster images to WebP using ffmpeg.
type ToWebPEngine struct{}

func (e *ToWebPEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted.webp")
	args := []string{
		"-y",
		"-i", inputPath,
		"-frames:v", "1",
		"-c:v", "libwebp",
		"-compression_level", "6",
		"-quality", "80",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg image-to-webp: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return outputPath, nil
}
