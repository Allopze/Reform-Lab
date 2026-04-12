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
	"regexp"
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
		// image.DecodeConfig does not support HEIF or SVG.
		// Try SVG header parsing and ffprobe as fallbacks.
		if w, h, ok := extractSVGDimensions(path); ok {
			m.Width = &w
			m.Height = &h
		} else if w, h, ok := extractDimensionsViaFFProbe(path); ok {
			m.Width = &w
			m.Height = &h
		}
		return m
	}
	m.Width = &cfg.Width
	m.Height = &cfg.Height
	return m
}

// extractSVGDimensions reads the root <svg> element and extracts width/height
// from viewBox or explicit width/height attributes. This avoids executing the SVG.
func extractSVGDimensions(path string) (w, h int, ok bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, false
	}
	content := string(data)
	if len(content) > 4096 {
		content = content[:4096]
	}

	// Try viewBox first: viewBox="minX minY width height"
	vbRe := regexp.MustCompile(`(?i)viewBox\s*=\s*["'][\s]*[\d.]+[\s,]+[\d.]+[\s,]+([\d.]+)[\s,]+([\d.]+)`)
	if m := vbRe.FindStringSubmatch(content); len(m) == 3 {
		wf, errW := strconv.ParseFloat(m[1], 64)
		hf, errH := strconv.ParseFloat(m[2], 64)
		if errW == nil && errH == nil && wf > 0 && hf > 0 {
			return int(wf), int(hf), true
		}
	}

	// Try explicit width/height attributes (px or unitless).
	widthRe := regexp.MustCompile(`(?i)<svg[^>]+width\s*=\s*["']([\d.]+)`)
	heightRe := regexp.MustCompile(`(?i)<svg[^>]+height\s*=\s*["']([\d.]+)`)
	wm := widthRe.FindStringSubmatch(content)
	hm := heightRe.FindStringSubmatch(content)
	if len(wm) == 2 && len(hm) == 2 {
		wf, errW := strconv.ParseFloat(wm[1], 64)
		hf, errH := strconv.ParseFloat(hm[1], 64)
		if errW == nil && errH == nil && wf > 0 && hf > 0 {
			return int(wf), int(hf), true
		}
	}

	return 0, 0, false
}

// extractDimensionsViaFFProbe uses ffprobe to get width/height for formats
// not supported by image.DecodeConfig (e.g. HEIF/HEIC).
func extractDimensionsViaFFProbe(path string) (w, h int, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(
		ctx,
		"ffprobe", "-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "v:0",
		path,
	).Output()
	if err != nil {
		return 0, 0, false
	}

	var result struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &result); err != nil || len(result.Streams) == 0 {
		return 0, 0, false
	}

	s := result.Streams[0]
	if s.Width > 0 && s.Height > 0 {
		return s.Width, s.Height, true
	}
	return 0, 0, false
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
