package audio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWaveformEngineCreatesPNGFromAudio(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "input.wav")
	writeTestAudioWAV(t, audioPath)

	outputPath, err := (&WaveformEngine{}).Execute(context.Background(), audioPath, dir, "png")
	if err != nil {
		t.Fatalf("audio waveform: %v", err)
	}
	assertPNGHeader(t, outputPath)
}

func TestWaveformEngineCreatesPNGFromVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "input.mp4")
	writeTestVideoWithAudio(t, videoPath)

	outputPath, err := (&WaveformEngine{}).Execute(context.Background(), videoPath, dir, "png")
	if err != nil {
		t.Fatalf("video waveform: %v", err)
	}
	assertPNGHeader(t, outputPath)
}

func writeTestAudioWAV(t *testing.T, outputPath string) {
	t.Helper()
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=4",
		"-codec:a", "pcm_s16le",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare test audio: %s: %v", strings.TrimSpace(string(out)), err)
	}
}

func assertPNGHeader(t *testing.T, outputPath string) {
	t.Helper()
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read waveform output: %v", err)
	}
	if len(data) < 8 || data[0] != 0x89 || data[1] != 0x50 || data[2] != 0x4e || data[3] != 0x47 {
		t.Fatalf("expected PNG header, got %v", data[:min(len(data), 8)])
	}
}
