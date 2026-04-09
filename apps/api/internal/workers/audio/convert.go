package audio

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConvertEngine converts audio formats and can also extract audio tracks from video inputs.
type ConvertEngine struct{}

func (e *ConvertEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted."+outputFormat)

	args := []string{"-i", inputPath, "-y", "-vn"}

	switch outputFormat {
	case "mp3":
		args = append(args, "-codec:a", "libmp3lame", "-q:a", "2")
	case "wav":
		args = append(args, "-codec:a", "pcm_s16le")
	case "ogg":
		args = append(args, "-codec:a", "libvorbis", "-q:a", "4")
	case "aac":
		args = append(args, "-codec:a", "aac", "-b:a", "160k")
	case "m4a":
		args = append(args, "-codec:a", "aac", "-b:a", "160k", "-movflags", "+faststart", "-f", "ipod")
	case "flac":
		args = append(args, "-codec:a", "flac")
	case "opus":
		args = append(args, "-codec:a", "libopus", "-b:a", "128k", "-f", "opus")
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
