package video

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestThumbnailZipEngineCreatesArchive(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "input.mp4")
	writePreviewTestVideo(t, videoPath)

	outputPath, err := (&ThumbnailZipEngine{}).Execute(context.Background(), videoPath, dir, "zip")
	if err != nil {
		t.Fatalf("video to thumbnails: %v", err)
	}
	archive, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer archive.Close()
	if len(archive.File) == 0 {
		t.Fatal("expected at least one thumbnail in the zip archive")
	}
}

func TestContactSheetEngineCreatesJPG(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "input.mp4")
	writePreviewTestVideo(t, videoPath)

	outputPath, err := (&ContactSheetEngine{}).Execute(context.Background(), videoPath, dir, "jpg")
	if err != nil {
		t.Fatalf("video contact sheet: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read contact sheet: %v", err)
	}
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		t.Fatalf("expected jpg header, got %q", data[:min(len(data), 2)])
	}
}

func writePreviewTestVideo(t *testing.T, outputPath string) {
	t.Helper()
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-f", "lavfi", "-i", "testsrc=size=160x120:rate=24",
		"-t", "6",
		"-c:v", "mpeg4",
		"-pix_fmt", "yuv420p",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare preview video: %s: %v", strings.TrimSpace(string(out)), err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
