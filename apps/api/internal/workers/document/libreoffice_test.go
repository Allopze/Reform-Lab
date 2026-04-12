package document

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLibreOfficeConvertReturnsFailureOutput(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("create fake bin dir: %v", err)
	}
	libreofficePath := filepath.Join(binDir, "libreoffice")
	script := "#!/bin/sh\nprintf '%s\n' 'convert /tmp/input.doc -> /tmp/output.docx using filter : Office Open XML Text'\nprintf '%s\n' 'Error: Please verify input parameters... (SfxBaseModel::impl_store failed)'\nexit 0\n"
	if err := os.WriteFile(libreofficePath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake libreoffice: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := runLibreOfficeConvert(context.Background(), "libreoffice doc-to-pdf", "pdf", filepath.Join(dir, "input.docx"), dir)
	if err == nil {
		t.Fatal("expected textual libreoffice error to be reported")
	}
	if !strings.Contains(err.Error(), "Please verify input parameters") {
		t.Fatalf("expected raw libreoffice failure details, got %v", err)
	}
}
