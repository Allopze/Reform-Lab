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
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const thumbnailMaxEdge = 320

// ThumbnailEngine renders a smaller preview image capped at 320px on its longest edge.
type ThumbnailEngine struct{}

func (e *ThumbnailEngine) Execute(_ context.Context, inputPath, outputDir, outputFormat string) (string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	decoded, _, err := img.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	thumb := resizeToFit(decoded, thumbnailMaxEdge)
	outputPath := filepath.Join(outputDir, "converted."+outputFormat)
	out, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	switch outputFormat {
	case "jpg":
		err = jpeg.Encode(out, thumb, &jpeg.Options{Quality: 82})
	case "png":
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		err = encoder.Encode(out, thumb)
	default:
		err = fmt.Errorf("unsupported output format: %s", outputFormat)
	}

	if err != nil {
		return "", fmt.Errorf("encode thumbnail %s: %w", outputFormat, err)
	}
	return outputPath, nil
}

func resizeToFit(source img.Image, maxEdge int) img.Image {
	bounds := source.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= maxEdge && height <= maxEdge {
		return source
	}

	scale := float64(maxEdge) / float64(width)
	if height > width {
		scale = float64(maxEdge) / float64(height)
	}
	targetWidth := int(float64(width) * scale)
	targetHeight := int(float64(height) * scale)
	if targetWidth < 1 {
		targetWidth = 1
	}
	if targetHeight < 1 {
		targetHeight = 1
	}

	dst := img.NewRGBA(img.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), source, bounds, draw.Over, nil)
	return dst
}
