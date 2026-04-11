package document

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runLibreOfficeConvert(ctx context.Context, label, convertTo, inputPath, outputDir string) error {
	profileDir, err := os.MkdirTemp(outputDir, "libreoffice-profile-")
	if err != nil {
		return fmt.Errorf("%s: create libreoffice profile dir: %w", label, err)
	}
	defer os.RemoveAll(profileDir)

	profileURL, err := libreOfficeProfileURL(profileDir)
	if err != nil {
		return fmt.Errorf("%s: build libreoffice profile url: %w", label, err)
	}

	cmd := exec.CommandContext(ctx,
		"libreoffice", "--headless", "-env:UserInstallation="+profileURL, "--convert-to", convertTo,
		"--outdir", outputDir, inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s: %w", label, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func ensureOutputFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("expected output %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("expected output %s to be a file", path)
	}
	return nil
}

func libreOfficeProfileURL(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return (&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(absPath),
	}).String(), nil
}
