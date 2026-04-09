package video

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	previewSampleSeconds = 6
	previewFrameCount    = 6
	previewFrameWidth    = 320
	previewClipSeconds   = 8
	previewClipWidth     = 960
)

// ThumbnailZipEngine extracts a small set of JPG thumbnails and packages them as a ZIP.
type ThumbnailZipEngine struct{}

func (e *ThumbnailZipEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	pattern := filepath.Join(outputDir, "thumb-%02d.jpg")
	args := []string{
		"-y",
		"-i", inputPath,
		"-t", fmt.Sprintf("%d", previewSampleSeconds),
		"-vf", fmt.Sprintf("fps=1,scale=%d:-1:flags=lanczos", previewFrameWidth),
		"-frames:v", fmt.Sprintf("%d", previewFrameCount),
		pattern,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg video-to-thumbnails: %s: %w", strings.TrimSpace(string(out)), err)
	}

	matches, err := filepath.Glob(filepath.Join(outputDir, "thumb-*.jpg"))
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("ffmpeg produced no thumbnails")
	}

	zipPath := filepath.Join(outputDir, "thumbnails.zip")
	return zipPath, createZip(zipPath, matches)
}

// ContactSheetEngine renders a single JPG contact sheet from sampled video frames.
type ContactSheetEngine struct{}

func (e *ContactSheetEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	outputPath := filepath.Join(outputDir, "contact-sheet.jpg")
	args := []string{
		"-y",
		"-i", inputPath,
		"-t", fmt.Sprintf("%d", previewSampleSeconds),
		"-vf", fmt.Sprintf("fps=1,scale=%d:-1:flags=lanczos,tile=3x2", previewFrameWidth),
		"-frames:v", "1",
		outputPath,
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg video-contact-sheet: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}

// PreviewClipEngine renders a short lightweight clip for quick previewing.
type PreviewClipEngine struct{}

func (e *PreviewClipEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	outputPath := filepath.Join(outputDir, "preview."+outputFormat)
	args := []string{
		"-y",
		"-ss", "0",
		"-i", inputPath,
		"-t", fmt.Sprintf("%d", previewClipSeconds),
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-vf", fmt.Sprintf("scale='min(%d,iw)':-2:flags=lanczos", previewClipWidth),
	}

	switch outputFormat {
	case "mp4":
		args = append(args,
			"-codec:v", "libx264",
			"-preset", "veryfast",
			"-crf", "24",
			"-codec:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
		)
	case "webm":
		args = append(args,
			"-codec:v", "libvpx-vp9",
			"-crf", "32",
			"-b:v", "0",
			"-codec:a", "libopus",
			"-b:a", "96k",
		)
	default:
		return "", fmt.Errorf("unsupported preview output format: %s", outputFormat)
	}

	args = append(args, outputPath)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg video preview: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return outputPath, nil
}

func createZip(zipPath string, files []string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		fw, err := w.Create(filepath.Base(path))
		if err != nil {
			return err
		}
		if _, err := fw.Write(data); err != nil {
			return err
		}
	}
	return nil
}
