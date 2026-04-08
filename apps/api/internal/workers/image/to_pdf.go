package image

import (
	"bytes"
	"context"
	"fmt"
	img "image"
	"image/jpeg"
	"os"
	"path/filepath"

	_ "image/gif"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ToPDFEngine wraps an image inside a single-page PDF.
// Uses a minimal PDF structure without external dependencies.
type ToPDFEngine struct{}

func (e *ToPDFEngine) Execute(_ context.Context, inputPath, outputDir, _ string) (string, error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return "", fmt.Errorf("open input: %w", err)
	}
	defer f.Close()

	decoded, _, err := img.Decode(f)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}

	bounds := decoded.Bounds()
	pixelW := bounds.Dx()
	pixelH := bounds.Dy()

	// Encode image to JPEG for embedding in PDF.
	var imgBuf bytes.Buffer
	if err := jpeg.Encode(&imgBuf, decoded, &jpeg.Options{Quality: 92}); err != nil {
		return "", fmt.Errorf("encode jpeg for pdf: %w", err)
	}
	jpegData := imgBuf.Bytes()

	// Scale to fit on a letter-size page (612x792 points) while preserving aspect ratio.
	pageW, pageH := 612.0, 792.0
	imgW, imgH := float64(pixelW), float64(pixelH)
	scale := pageW / imgW
	if imgH*scale > pageH {
		scale = pageH / imgH
	}
	dispW := imgW * scale
	dispH := imgH * scale

	pdfBytes := buildMinimalPDF(jpegData, pixelW, pixelH, dispW, dispH, pageW, pageH)

	outputPath := filepath.Join(outputDir, "converted.pdf")
	if err := os.WriteFile(outputPath, pdfBytes, 0o644); err != nil {
		return "", fmt.Errorf("write pdf: %w", err)
	}

	return outputPath, nil
}

func buildMinimalPDF(jpegData []byte, pixelW, pixelH int, dispW, dispH, pageW, pageH float64) []byte {
	var b bytes.Buffer

	b.WriteString("%PDF-1.4\n")

	// Object 1: Catalog
	obj1Offset := b.Len()
	b.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	obj2Offset := b.Len()
	b.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: Page
	obj3Offset := b.Len()
	fmt.Fprintf(&b, "3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.2f %.2f] /Contents 4 0 R /Resources << /XObject << /Img 5 0 R >> >> >>\nendobj\n", pageW, pageH)

	// Object 4: Content stream (draw image centered)
	streamContent := fmt.Sprintf("q\n%.2f 0 0 %.2f 0 0 cm\n/Img Do\nQ\n", dispW, dispH)
	obj4Offset := b.Len()
	fmt.Fprintf(&b, "4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(streamContent), streamContent)

	// Object 5: Image XObject
	obj5Offset := b.Len()
	fmt.Fprintf(&b, "5 0 obj\n<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length %d >>\nstream\n",
		pixelW, pixelH, len(jpegData))
	b.Write(jpegData)
	b.WriteString("\nendstream\nendobj\n")

	// Cross-reference table
	xrefOffset := b.Len()
	fmt.Fprintf(&b, "xref\n0 6\n0000000000 65535 f \n")
	fmt.Fprintf(&b, "%010d 00000 n \n", obj1Offset)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj2Offset)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj3Offset)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj4Offset)
	fmt.Fprintf(&b, "%010d 00000 n \n", obj5Offset)

	fmt.Fprintf(&b, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	return b.Bytes()
}
