package ingestion

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	pdfMetadataTimeout = 5 * time.Second
	avMetadataTimeout  = 8 * time.Second
)

// ExtractMetadata gathers format-specific metadata from a file on disk.
func ExtractMetadata(ctx context.Context, path string, format domain.DetectedFormat) (domain.FileMetadata, error) {
	switch format.Family {
	case domain.FamilyPDF:
		return extractPDFMetadata(ctx, path)
	case domain.FamilyImage:
		return extractImageMetadata(path), nil
	case domain.FamilyAudio, domain.FamilyVideo:
		return extractAVMetadata(ctx, path)
	default:
		return domain.FileMetadata{}, nil
	}
}

func extractPDFMetadata(ctx context.Context, path string) (domain.FileMetadata, error) {
	m := domain.FileMetadata{}
	cmdCtx, cancel := context.WithTimeout(ctx, pdfMetadataTimeout)
	defer cancel()

	out, err := exec.CommandContext(cmdCtx, "pdfinfo", path).Output()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return m, context.DeadlineExceeded
		}
		if errors.Is(err, context.Canceled) || errors.Is(cmdCtx.Err(), context.Canceled) {
			return m, context.Canceled
		}
		return m, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "Pages:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Pages:"))
			if n, err := strconv.Atoi(val); err == nil {
				m.Pages = &n
			}
		}
		if strings.HasPrefix(line, "Encrypted:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Encrypted:"))
			if val == "yes" {
				m.IsProtected = true
			}
		}
	}
	return m, nil
}

func extractImageMetadata(path string) domain.FileMetadata {
	m := domain.FileMetadata{}
	f, err := os.Open(path)
	if err != nil {
		return m
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return m
	}
	m.Width = &cfg.Width
	m.Height = &cfg.Height
	return m
}

// ffprobeResult is the minimal structure we parse from ffprobe JSON output.
type ffprobeResult struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func extractAVMetadata(ctx context.Context, path string) (domain.FileMetadata, error) {
	m := domain.FileMetadata{}
	cmdCtx, cancel := context.WithTimeout(ctx, avMetadataTimeout)
	defer cancel()

	out, err := exec.CommandContext(
		cmdCtx,
		"ffprobe", "-v", "quiet",
		"-print_format", "json",
		"-show_format",
		path,
	).Output()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return m, context.DeadlineExceeded
		}
		if errors.Is(err, context.Canceled) || errors.Is(cmdCtx.Err(), context.Canceled) {
			return m, context.Canceled
		}
		return m, nil
	}
	var result ffprobeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return m, nil
	}
	if result.Format.Duration != "" {
		d, err := strconv.ParseFloat(result.Format.Duration, 64)
		if err == nil {
			m.DurationSec = &d
		}
	}
	return m, nil
}
