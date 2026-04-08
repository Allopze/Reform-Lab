package video

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ToGIFEngine converts video to animated GIF using FFmpeg.
// Limited to first 30 seconds, 480px wide, 10 fps.
type ToGIFEngine struct{}

func (e *ToGIFEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted.gif")

	args := []string{
		"-i", inputPath,
		"-y",
		"-t", "30",
		"-vf", "fps=10,scale=480:-1:flags=lanczos",
		"-loop", "0",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg video-to-gif: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}
