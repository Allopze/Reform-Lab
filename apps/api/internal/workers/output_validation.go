package workers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"unicode/utf8"

	"github.com/gabriel-vasile/mimetype"
)

const outputValidationSampleLimit = 128 * 1024

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

func validateOutputArtifact(path, expectedFormat string) (os.FileInfo, error) {
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

	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			return nil
		}
	}

	return fmt.Errorf("zip output has no files")
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
