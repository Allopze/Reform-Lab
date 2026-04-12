package pdf

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToDocxEngineUsesPdf2DocxCLI(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}
	pdf2docxPath := filepath.Join(binDir, "pdf2docx")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$3\"\n"
	if err := os.WriteFile(pdf2docxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake pdf2docx: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	inputPath := filepath.Join(dir, "input.pdf")
	if err := os.WriteFile(inputPath, []byte("pdf"), 0o600); err != nil {
		t.Fatalf("write fake input: %v", err)
	}

	outputPath, err := (&ToDocxEngine{}).Execute(context.Background(), inputPath, dir, "docx")
	if err != nil {
		t.Fatalf("execute pdf2docx engine: %v", err)
	}
	if outputPath != filepath.Join(dir, "converted.docx") {
		t.Fatalf("expected converted.docx output path, got %s", outputPath)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read fake docx output: %v", err)
	}
	if !strings.Contains(string(data), "convert") || !strings.Contains(string(data), inputPath) {
		t.Fatalf("expected CLI arguments in output, got %q", string(data))
	}
}

func TestToDocxEngineRequiresOutputFile(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}
	pdf2docxPath := filepath.Join(binDir, "pdf2docx")
	script := "#!/bin/sh\nprintf '%s\\n' 'conversion finished without writing output'\nexit 0\n"
	if err := os.WriteFile(pdf2docxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake pdf2docx: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	inputPath := filepath.Join(dir, "input.pdf")
	if err := os.WriteFile(inputPath, []byte("pdf"), 0o600); err != nil {
		t.Fatalf("write fake input: %v", err)
	}

	_, err := (&ToDocxEngine{}).Execute(context.Background(), inputPath, dir, "docx")
	if err == nil {
		t.Fatal("expected missing output to be reported")
	}
	if !strings.Contains(err.Error(), "output") {
		t.Fatalf("expected missing output error details, got %v", err)
	}
}
