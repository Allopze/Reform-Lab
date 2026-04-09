package image

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToAVIFEngine converts raster images to AVIF using ffmpeg.
type ToAVIFEngine struct{}

func (e *ToAVIFEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted.avif")
	args := []string{
		"-y",
		"-i", inputPath,
		"-frames:v", "1",
		"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2:flags=lanczos",
		"-c:v", "libaom-av1",
		"-still-picture", "1",
		"-crf", "30",
		"-b:v", "0",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg image-to-avif: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return outputPath, nil
}
