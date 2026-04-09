package image

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// WebVariantEngine renders a capped-size still image optimized for web delivery.
type WebVariantEngine struct {
	MaxEdge int
}

func NewWebVariantEngine(maxEdge int) *WebVariantEngine {
	return &WebVariantEngine{MaxEdge: maxEdge}
}

func (e *WebVariantEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	outputPath := filepath.Join(outputDir, "converted."+outputFormat)
	filter := fmt.Sprintf("scale=w='min(iw,%d)':h='min(ih,%d)':force_original_aspect_ratio=decrease:flags=lanczos", e.MaxEdge, e.MaxEdge)
	args := []string{
		"-y",
		"-i", inputPath,
		"-an",
		"-sn",
		"-dn",
		"-frames:v", "1",
		"-map_metadata", "-1",
	}

	switch outputFormat {
	case "jpg":
		args = append(args,
			"-vf", filter,
			"-c:v", "mjpeg",
			"-q:v", "3",
		)
	case "webp":
		args = append(args,
			"-vf", filter,
			"-c:v", "libwebp",
			"-compression_level", "6",
			"-quality", "82",
		)
	case "avif":
		args = append(args,
			"-vf", filter+",scale=trunc(iw/2)*2:trunc(ih/2)*2",
			"-c:v", "libaom-av1",
			"-still-picture", "1",
			"-crf", "28",
			"-b:v", "0",
		)
	default:
		return "", fmt.Errorf("unsupported web variant output format: %s", outputFormat)
	}

	args = append(args, outputPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg image web variant: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}
