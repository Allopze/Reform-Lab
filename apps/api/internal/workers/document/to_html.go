package document

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ToHTMLEngine converts LibreOffice-supported office inputs to simple HTML.
type ToHTMLEngine struct {
	PackageCompanions bool
}

func (e *ToHTMLEngine) Execute(ctx context.Context, inputPath, outputDir, _ string) (string, error) {
	// If the input is HTML, sanitize it before passing to LibreOffice.
	if looksLikeHTML(inputPath) {
		safePath := filepath.Join(outputDir, "safe-input.html")
		if err := copyAndSanitizeHTML(inputPath, safePath); err != nil {
			return "", fmt.Errorf("sanitize html input: %w", err)
		}
		inputPath = safePath
	}

	if err := runLibreOfficeConvert(ctx, "libreoffice doc-to-html", "html", inputPath, outputDir); err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	outputPath := filepath.Join(outputDir, base+".html")
	if err := ensureOutputFile(outputPath); err != nil {
		return "", err
	}

	// Sanitize the output HTML to strip any remote references or scripts.
	outData, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read html output: %w", err)
	}
	if err := os.WriteFile(outputPath, sanitizeHTMLBytes(outData), 0o644); err != nil {
		return "", fmt.Errorf("write sanitized html output: %w", err)
	}

	if e.PackageCompanions {
		packagedPath, err := packageHTMLWithCompanions(outputPath, outputDir, base)
		if err != nil {
			return "", err
		}
		if packagedPath != "" {
			return packagedPath, nil
		}
	}

	return outputPath, nil
}

func packageHTMLWithCompanions(outputPath, outputDir, base string) (string, error) {
	companions, err := htmlCompanionFiles(outputDir, outputPath)
	if err != nil {
		return "", err
	}
	if len(companions) == 0 {
		return "", nil
	}

	zipPath := filepath.Join(outputDir, base+".zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("create html package: %w", err)
	}
	defer zipFile.Close()

	writer := zip.NewWriter(zipFile)
	if err := addFileToZip(writer, outputPath, filepath.Base(outputPath)); err != nil {
		writer.Close()
		return "", err
	}
	for _, companion := range companions {
		rel, err := filepath.Rel(outputDir, companion)
		if err != nil {
			writer.Close()
			return "", fmt.Errorf("resolve html companion path: %w", err)
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, "/") {
			writer.Close()
			return "", fmt.Errorf("unsafe html companion path %q", rel)
		}
		if err := addFileToZip(writer, companion, rel); err != nil {
			writer.Close()
			return "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("finalize html package: %w", err)
	}
	return zipPath, nil
}

func htmlCompanionFiles(outputDir, outputPath string) ([]string, error) {
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve html output path: %w", err)
	}
	files := make([]string, 0)
	err = filepath.WalkDir(outputDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		pathAbs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if pathAbs == outputAbs {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list html companion assets: %w", err)
	}
	return files, nil
}

func addFileToZip(writer *zip.Writer, filePath, entryName string) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat html package entry %q: %w", entryName, err)
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("create html package header %q: %w", entryName, err)
	}
	header.Name = filepath.ToSlash(entryName)
	header.Method = zip.Deflate

	entry, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create html package entry %q: %w", entryName, err)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open html package entry %q: %w", entryName, err)
	}
	defer file.Close()
	if _, err := io.Copy(entry, file); err != nil {
		return fmt.Errorf("write html package entry %q: %w", entryName, err)
	}
	return nil
}
