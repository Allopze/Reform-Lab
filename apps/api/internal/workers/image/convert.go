package image

import (
	"context"
	"fmt"
	img "image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	_ "image/gif"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ConvertEngine converts between image formats using Go's stdlib.
type ConvertEngine struct{}

func (e *ConvertEngine) Execute(_ context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	decoded, _, err := img.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	outputPath := filepath.Join(outputDir, "converted."+outputFormat)
	out, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	switch outputFormat {
	case "png":
		err = png.Encode(out, decoded)
	case "jpg":
		err = jpeg.Encode(out, decoded, &jpeg.Options{Quality: 90})
	default:
		err = fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	if err != nil {
		return "", fmt.Errorf("encode %s: %w", outputFormat, err)
	}
	return outputPath, nil
}
