package audio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertEngineExtractsAudioFromVideo(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "input.mp4")
	writeTestVideoWithAudio(t, videoPath)

	engine := &ConvertEngine{}
	mp3Path, err := engine.Execute(context.Background(), videoPath, dir, "mp3")
	if err != nil {
		t.Fatalf("video to mp3: %v", err)
	}
	info, err := os.Stat(mp3Path)
	if err != nil {
		t.Fatalf("stat mp3: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("expected non-empty mp3 output")
	}

	wavPath, err := engine.Execute(context.Background(), videoPath, dir, "wav")
	if err != nil {
		t.Fatalf("video to wav: %v", err)
	}
	data, err := os.ReadFile(wavPath)
	if err != nil {
		t.Fatalf("read wav: %v", err)
	}
	if !strings.HasPrefix(string(data[:4]), "RIFF") {
		t.Fatalf("expected wav RIFF header, got %q", data[:4])
	}

	aacPath, err := engine.Execute(context.Background(), videoPath, dir, "aac")
	if err != nil {
		t.Fatalf("video to aac: %v", err)
	}
	aacData, err := os.ReadFile(aacPath)
	if err != nil {
		t.Fatalf("read aac: %v", err)
	}
	if len(aacData) < 2 || aacData[0] != 0xff || (aacData[1]&0xf0) != 0xf0 {
		t.Fatalf("expected ADTS AAC header, got %v", aacData[:min(len(aacData), 2)])
	}

	flacPath, err := engine.Execute(context.Background(), videoPath, dir, "flac")
	if err != nil {
		t.Fatalf("video to flac: %v", err)
	}
	flacData, err := os.ReadFile(flacPath)
	if err != nil {
		t.Fatalf("read flac: %v", err)
	}
	if !strings.HasPrefix(string(flacData[:4]), "fLaC") {
		t.Fatalf("expected FLAC header, got %q", flacData[:4])
	}

	m4aPath, err := engine.Execute(context.Background(), videoPath, dir, "m4a")
	if err != nil {
		t.Fatalf("video to m4a: %v", err)
	}
	m4aData, err := os.ReadFile(m4aPath)
	if err != nil {
		t.Fatalf("read m4a: %v", err)
	}
	if len(m4aData) < 12 || string(m4aData[4:8]) != "ftyp" {
		t.Fatalf("expected MP4 family header, got %q", m4aData[:min(len(m4aData), 12)])
	}

	opusPath, err := engine.Execute(context.Background(), videoPath, dir, "opus")
	if err != nil {
		t.Fatalf("video to opus: %v", err)
	}
	opusData, err := os.ReadFile(opusPath)
	if err != nil {
		t.Fatalf("read opus: %v", err)
	}
	if !strings.HasPrefix(string(opusData[:4]), "OggS") {
		t.Fatalf("expected Ogg Opus header, got %q", opusData[:4])
	}
}

func writeTestVideoWithAudio(t *testing.T, outputPath string) {
	t.Helper()
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=24",
		"-f", "lavfi", "-i", "sine=frequency=1000:duration=6",
		"-t", "6",
		"-shortest",
		"-c:v", "mpeg4",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare test video: %s: %v", strings.TrimSpace(string(out)), err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
