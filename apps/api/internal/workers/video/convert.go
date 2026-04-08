package video

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConvertEngine converts between video formats using FFmpeg.
type ConvertEngine struct{}

func (e *ConvertEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted."+outputFormat)

	args := []string{"-i", inputPath, "-y"}

	switch outputFormat {
	case "mp4":
		args = append(args, "-codec:v", "libx264", "-preset", "medium", "-crf", "23", "-codec:a", "aac", "-b:a", "128k")
	case "webm":
		args = append(args, "-codec:v", "libvpx-vp9", "-crf", "30", "-b:v", "0", "-codec:a", "libopus")
	default:
		return "", fmt.Errorf("unsupported video output format: %s", outputFormat)
	}

	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg video: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}
