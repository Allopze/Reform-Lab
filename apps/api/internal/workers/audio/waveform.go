package audio

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const waveformSize = "1280x360"

// WaveformEngine renders a static PNG waveform from the primary audio stream.
type WaveformEngine struct{}

func (e *WaveformEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "waveform.png")
	args := []string{
		"-y",
		"-i", inputPath,
		"-filter_complex", fmt.Sprintf("[0:a:0]showwavespic=s=%s:colors=0xE86F50:scale=sqrt:draw=full[v]", waveformSize),
		"-map", "[v]",
		"-frames:v", "1",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg waveform: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}
