package document

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// PresentationToImagesEngine renders slides to PDF first, then rasterizes them page by page.
type PresentationToImagesEngine struct{}

func (e *PresentationToImagesEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	tmpDir, err := os.MkdirTemp(outputDir, "presentation-pdf-")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	pdfPath, err := (&ToPDFEngine{}).Execute(ctx, inputPath, tmpDir, "pdf")
	if err != nil {
		return "", fmt.Errorf("presentation to pdf: %w", err)
	}

	return renderPresentationSlides(ctx, pdfPath, outputDir, outputFormat)
}

func renderPresentationSlides(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	flag := "-png"
	ext := "png"
	if outputFormat == "jpg" {
		flag = "-jpeg"
		ext = "jpg"
	}

	prefixPath := filepath.Join(outputDir, "slide")
	cmd := exec.CommandContext(ctx, "pdftoppm", flag, "-r", "200", inputPath, prefixPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("pdftoppm presentation: %s: %w", strings.TrimSpace(string(out)), err)
	}

	pattern := filepath.Join(outputDir, "slide-*"+"."+ext)
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		pattern = filepath.Join(outputDir, "slide*"+"."+ext)
		matches, _ = filepath.Glob(pattern)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("pdftoppm produced no slide images")
	}
	sort.Strings(matches)
	if len(matches) == 1 {
		return matches[0], nil
	}

	zipPath := filepath.Join(outputDir, "slides.zip")
	return zipPath, createSlideZip(zipPath, matches)
}

func createSlideZip(zipPath string, files []string) error {
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
