package pdf

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

// ToImagesEngine converts PDF pages to images using Poppler's pdftoppm.
type ToImagesEngine struct{}

func (e *ToImagesEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	flag := "-png"
	ext := "png"
	if outputFormat == "jpg" {
		flag = "-jpeg"
		ext = "jpg"
	}

	matches, err := renderPDFPages(ctx, inputPath, outputDir, "page", flag, ext)
	if err != nil {
		return "", err
	}

	// Single page: return the image directly.
	if len(matches) == 1 {
		return matches[0], nil
	}

	// Multi-page: create a ZIP.
	zipPath := filepath.Join(outputDir, "pages.zip")
	return zipPath, createZip(zipPath, matches)
}

func renderPDFPages(ctx context.Context, inputPath, outputDir, prefix, flag, ext string) ([]string, error) {
	prefixPath := filepath.Join(outputDir, prefix)
	cmd := exec.CommandContext(ctx, "pdftoppm", flag, "-r", "200", inputPath, prefixPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pdftoppm: %s: %w", strings.TrimSpace(string(out)), err)
	}

	pattern := filepath.Join(outputDir, prefix+"-*."+ext)
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		pattern = filepath.Join(outputDir, prefix+"*."+ext)
		matches, _ = filepath.Glob(pattern)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("pdftoppm produced no output files")
	}
	sort.Strings(matches)
	return matches, nil
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
