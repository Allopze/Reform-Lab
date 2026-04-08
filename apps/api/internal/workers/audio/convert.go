package audio

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConvertEngine converts between audio formats using FFmpeg.
type ConvertEngine struct{}

func (e *ConvertEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted."+outputFormat)

	args := []string{"-i", inputPath, "-y"}

	switch outputFormat {
	case "mp3":
		args = append(args, "-codec:a", "libmp3lame", "-q:a", "2")
	case "wav":
		args = append(args, "-codec:a", "pcm_s16le")
	case "ogg":
		args = append(args, "-codec:a", "libvorbis", "-q:a", "4")
	default:
		return "", fmt.Errorf("unsupported audio output format: %s", outputFormat)
	}

	args = append(args, outputPath)

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg audio: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}
