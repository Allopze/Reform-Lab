package workers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/gabriel-vasile/mimetype"
)

const outputValidationSampleLimit = 128 * 1024
const maxZipOutputEntries = 2000

var allowedOutputMIMEs = map[string][]string{
	"pdf":  {"application/pdf"},
	"html": {"text/html"},
	"jpg":  {"image/jpeg"},
	"png":  {"image/png"},
	"webp": {"image/webp"},
	"avif": {"image/avif"},
	"gif":  {"image/gif"},
	"mp3":  {"audio/mpeg"},
	"wav":  {"audio/wav"},
	"ogg":  {"audio/ogg", "application/ogg"},
	"aac":  {"audio/aac"},
	"m4a":  {"audio/mp4", "audio/x-m4a", "audio/x-mp4a"},
	"flac": {"audio/flac"},
	"opus": {"audio/opus", "audio/ogg", "application/ogg"},
	"mp4":  {"video/mp4"},
	"webm": {"video/webm"},
}

func validateOutputArtifact(path, expectedFormat string, inputSize int64) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("output file missing: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("output path is not a file")
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("output file missing or empty")
	}

	// Validate minimum reasonable size based on format
	if err := validateMinimumOutputSize(info.Size(), expectedFormat, inputSize); err != nil {
		return nil, err
	}

	switch expectedFormat {
	case "json":
		if err := validateJSONOutput(path); err != nil {
			return nil, err
		}
	case "csv":
		if err := validateCSVOutput(path); err != nil {
			return nil, err
		}
	case "txt", "md":
		if err := validateTextOutput(path, expectedFormat); err != nil {
			return nil, err
		}
	case "docx":
		if err := validateOOXMLOutput(path, "word/document.xml"); err != nil {
			return nil, err
		}
	case "xlsx":
		if err := validateOOXMLOutput(path, "xl/workbook.xml"); err != nil {
			return nil, err
		}
	case "zip":
		if err := validateZipOutput(path); err != nil {
			return nil, err
		}
	default:
		if err := validateBinaryOutputFormat(path, expectedFormat); err != nil {
			return nil, err
		}
	}

	return info, nil
}

func validateBinaryOutputFormat(path, expectedFormat string) error {
	detected, err := mimetype.DetectFile(path)
	if err != nil {
		return fmt.Errorf("detect output mime: %w", err)
	}

	mime := normalizeOutputMIME(detected.String())
	allowed := allowedOutputMIMEs[expectedFormat]
	if len(allowed) == 0 {
		return nil
	}
	if !slices.Contains(allowed, mime) {
		return fmt.Errorf("output format mismatch: expected %s, detected %s", expectedFormat, mime)
	}
	return nil
}

func validateTextOutput(path, format string) error {
	sample, err := readOutputValidationSample(path)
	if err != nil {
		return fmt.Errorf("read %s output: %w", format, err)
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return fmt.Errorf("%s output contains binary bytes", format)
	}
	if !utf8.Valid(sample) {
		return fmt.Errorf("%s output is not valid UTF-8", format)
	}
	return nil
}

func validateJSONOutput(path string) error {
	if err := validateTextOutput(path, "json"); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open json output: %w", err)
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	var payload interface{}
	if err := dec.Decode(&payload); err != nil {
		return fmt.Errorf("parse json output: %w", err)
	}

	var trailing interface{}
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("json output contains trailing content")
		}
		return fmt.Errorf("parse json output: %w", err)
	}

	return nil
}

func validateCSVOutput(path string) error {
	if err := validateTextOutput(path, "csv"); err != nil {
		return err
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open csv output: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	record, err := reader.Read()
	if err == io.EOF {
		return fmt.Errorf("csv output is empty")
	}
	if err != nil {
		return fmt.Errorf("parse csv output: %w", err)
	}
	if len(record) == 0 {
		return fmt.Errorf("csv output has no columns")
	}
	return nil
}

func validateZipOutput(path string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open zip output: %w", err)
	}
	defer reader.Close()

	files := 0
	for _, file := range reader.File {
		if err := validateZipEntryName(file.Name); err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.UncompressedSize64 == 0 {
			return fmt.Errorf("zip output contains empty file %q", file.Name)
		}
		files++
		if files > maxZipOutputEntries {
			return fmt.Errorf("zip output contains too many files")
		}
	}
	if files == 0 {
		return fmt.Errorf("zip output has no files")
	}
	return nil
}

func validateZipEntryName(name string) error {
	cleaned := path.Clean(name)
	if name == "" || cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." || path.IsAbs(name) {
		return fmt.Errorf("zip output contains unsafe path %q", name)
	}
	if strings.Contains(name, "\\") || strings.Contains(name, "\x00") {
		return fmt.Errorf("zip output contains unsafe path %q", name)
	}
	return nil
}

func validateOOXMLOutput(path, requiredEntry string) error {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("open office output: %w", err)
	}
	defer reader.Close()

	hasContentTypes := false
	hasRequiredEntry := false
	for _, file := range reader.File {
		switch file.Name {
		case "[Content_Types].xml":
			hasContentTypes = true
		case requiredEntry:
			hasRequiredEntry = true
		}
	}

	if !hasContentTypes || !hasRequiredEntry {
		return fmt.Errorf("office output is missing required document parts")
	}
	return nil
}

func readOutputValidationSample(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(io.LimitReader(file, outputValidationSampleLimit))
}

func normalizeOutputMIME(mime string) string {
	base, _, _ := strings.Cut(mime, ";")
	mime = strings.TrimSpace(base)

	switch mime {
	case "application/x-pdf":
		return "application/pdf"
	case "application/x-zip", "application/x-zip-compressed":
		return "application/zip"
	case "audio/mp3", "audio/x-mpeg":
		return "audio/mpeg"
	case "audio/x-wav", "audio/vnd.wave", "audio/wave":
		return "audio/wav"
	case "audio/x-ogg":
		return "application/ogg"
	default:
		return mime
	}
}

// minimumOutputSizes defines the minimum expected output size in bytes per format.
// Formats not listed use a generic 10-byte minimum.
var minimumOutputSizes = map[string]int64{
	"pdf":  256, // PDFs have headers and object definitions
	"docx": 256, // OOXML requires multiple internal files
	"xlsx": 256, // OOXML requires multiple internal files
	"pptx": 256, // OOXML requires multiple internal files
	"html": 16,  // HTML requires at least basic structure
	"jpg":  50,  // JPEG has headers and quantization tables
	"png":  50,  // PNG has headers and IHDR chunks
	"webp": 50,  // WebP has RIFF headers
	"avif": 50,  // AVIF has ftyp and meta boxes
	"gif":  50,  // GIF has header and image descriptor
	"mp4":  256, // MP4 has ftyp, moov boxes
	"webm": 256, // WebM has EBML headers
	"mp3":  64,  // MP3 has ID3 or frame headers
	"wav":  44,  // WAV minimum is header size
	"ogg":  64,  // OGG has page headers
	"aac":  64,  // AAC has ADTS headers
	"m4a":  256, // M4A is MP4 container
	"flac": 64,  // FLAC has streaminfo block
	"opus": 64,  // Opus has OGG container
	"zip":  22,  // ZIP minimum is end of central directory
	"csv":  5,   // CSV needs at least one field
	"txt":  5,   // Text needs at least some content
	"md":   5,   // Markdown needs at least some content
	"json": 5,   // JSON needs at least {} or []
}

func validateMinimumOutputSize(outputSize int64, expectedFormat string, inputSize int64) error {
	minSize, ok := minimumOutputSizes[expectedFormat]
	if !ok {
		minSize = 10 // Generic minimum for unknown formats
	}

	if outputSize < minSize {
		return fmt.Errorf("output file is suspiciously small: %d bytes for format %s (minimum expected: %d bytes)", outputSize, expectedFormat, minSize)
	}

	// For lossless conversions, output should not be smaller than 1% of input
	// This catches cases where conversion silently produced garbage
	if inputSize > 0 && outputSize < inputSize/100 {
		return fmt.Errorf("output file is less than 1%% of input size: %d bytes output vs %d bytes input", outputSize, inputSize)
	}

	return nil
}
