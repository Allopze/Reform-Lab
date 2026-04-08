package ingestion

import (
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ExtractMetadata gathers format-specific metadata from a file on disk.
func ExtractMetadata(path string, format domain.DetectedFormat) domain.FileMetadata {
	switch format.Family {
	case domain.FamilyPDF:
		return extractPDFMetadata(path)
	case domain.FamilyImage:
		return extractImageMetadata(path)
	case domain.FamilyAudio, domain.FamilyVideo:
		return extractAVMetadata(path)
	default:
		return domain.FileMetadata{}
	}
}

func extractPDFMetadata(path string) domain.FileMetadata {
	m := domain.FileMetadata{}
	out, err := exec.Command("pdfinfo", path).Output()
	if err != nil {
		return m
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
	return m
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

func extractAVMetadata(path string) domain.FileMetadata {
	m := domain.FileMetadata{}
	out, err := exec.Command(
		"ffprobe", "-v", "quiet",
		"-print_format", "json",
		"-show_format",
		path,
	).Output()
	if err != nil {
		return m
	}
	var result ffprobeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return m
	}
	if result.Format.Duration != "" {
		d, err := strconv.ParseFloat(result.Format.Duration, 64)
		if err == nil {
			m.DurationSec = &d
		}
	}
	return m
}
