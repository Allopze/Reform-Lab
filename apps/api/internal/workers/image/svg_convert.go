package image

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	workerDocument "github.com/allopze/reform-lab/apps/api/internal/workers/document"
	"golang.org/x/net/html"
)

// SVGConvertEngine rasterizes bitmap outputs and keeps vector PDF export via librsvg.
type SVGConvertEngine struct{}

func (e *SVGConvertEngine) Execute(ctx context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	sanitizedInputPath, cleanup, err := prepareSanitizedSVG(inputPath, outputDir)
	if err != nil {
		return "", err
	}
	defer cleanup()

	switch outputFormat {
	case "png":
		outputPath := filepath.Join(outputDir, "converted.png")
		if err := renderSVG(ctx, sanitizedInputPath, outputPath); err != nil {
			return "", err
		}
		return outputPath, nil
	case "webp":
		tmpDir, err := os.MkdirTemp(outputDir, "svg-webp-")
		if err != nil {
			return "", fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		rasterPath := filepath.Join(tmpDir, "rendered.png")
		if err := renderSVG(ctx, sanitizedInputPath, rasterPath); err != nil {
			return "", err
		}
		return (&ToWebPEngine{}).Execute(ctx, rasterPath, outputDir, outputFormat)
	case "pdf":
		outputPath := filepath.Join(outputDir, "converted.pdf")
		if err := renderSVGToPDF(ctx, sanitizedInputPath, outputPath); err != nil {
			return "", err
		}
		return outputPath, nil
	default:
		return "", fmt.Errorf("unsupported svg output format: %s", outputFormat)
	}
}

func prepareSanitizedSVG(inputPath, outputDir string) (string, func(), error) {
	tmpDir, err := os.MkdirTemp(outputDir, "svg-sanitized-")
	if err != nil {
		return "", func() {}, fmt.Errorf("create sanitized svg temp dir: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	content, err := os.ReadFile(inputPath)
	if err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("read svg: %w", err)
	}

	sanitizedPath := filepath.Join(tmpDir, "input.svg")
	sanitizedContent := workerDocument.SanitizeHTMLBytes(content)
	sanitizedContent = extractSVGDocument(sanitizedContent)
	if len(sanitizedContent) == 0 {
		cleanup()
		return "", func() {}, fmt.Errorf("sanitize svg: missing svg root after sanitization")
	}
	if err := os.WriteFile(sanitizedPath, sanitizedContent, 0o600); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write sanitized svg: %w", err)
	}

	return sanitizedPath, cleanup, nil
}

func extractSVGDocument(content []byte) []byte {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil
	}

	svg := findSVGNode(doc)
	if svg == nil {
		return nil
	}

	var out bytes.Buffer
	if err := html.Render(&out, svg); err != nil {
		return nil
	}
	return out.Bytes()
}

func findSVGNode(node *html.Node) *html.Node {
	if node.Type == html.ElementNode {
		name := strings.ToLower(node.Data)
		if name == "svg" || strings.HasSuffix(name, ":svg") {
			return node
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findSVGNode(child); found != nil {
			return found
		}
	}
	return nil
}

func renderSVG(ctx context.Context, inputPath, outputPath string) error {
	cmd := exec.CommandContext(ctx,
		"ffmpeg", "-y",
		"-i", inputPath,
		"-frames:v", "1",
		outputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg svg render: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func renderSVGToPDF(ctx context.Context, inputPath, outputPath string) error {
	cmd := exec.CommandContext(ctx,
		"rsvg-convert",
		"--format=pdf",
		"--output", outputPath,
		inputPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("rsvg-convert svg to pdf: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
