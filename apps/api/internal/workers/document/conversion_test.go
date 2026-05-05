package document

import (
	"archive/zip"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func documentFixturePath(parts ...string) string {
	path := []string{"..", "..", "..", "tests", "fixtures"}
	path = append(path, parts...)
	return filepath.Join(path...)
}

func requireRuntime(t *testing.T, bins ...string) {
	t.Helper()
	for _, bin := range bins {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available", bin)
		}
	}
}

func readZipEntries(t *testing.T, zipPath string) []string {
	t.Helper()
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()

	entries := make([]string, 0, len(r.File))
	for _, file := range r.File {
		entries = append(entries, file.Name)
	}
	return entries
}

func TestToPDFEngineConvertsTextFile(t *testing.T) {
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(inputPath, []byte("Hola\n\nLinea 2\n"), 0o644); err != nil {
		t.Fatalf("write txt: %v", err)
	}

	engine := &ToPDFEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "pdf")
	if err != nil {
		t.Fatalf("txt to pdf: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if !strings.HasPrefix(string(data[:5]), "%PDF-") {
		t.Fatalf("expected pdf header, got %q", data[:5])
	}
}

func TestToPDFEngineConvertsLegacyDocFixture(t *testing.T) {
	requireRuntime(t, "libreoffice")

	dir := t.TempDir()
	outputPath, err := (&ToPDFEngine{}).Execute(
		context.Background(),
		documentFixturePath("doc", "valid-basic.doc"),
		dir,
		"pdf",
	)
	if err != nil {
		t.Fatalf("legacy doc to pdf: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if !strings.HasPrefix(string(data[:5]), "%PDF-") {
		t.Fatalf("expected pdf header, got %q", data[:5])
	}
}

func TestToHTMLEngineConvertsDocxFile(t *testing.T) {
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("Titulo\n\ntexto base\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cmd := exec.Command("libreoffice", "--headless", "--convert-to", "docx", "--outdir", dir, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare docx: %s: %v", strings.TrimSpace(string(out)), err)
	}

	engine := &ToHTMLEngine{}
	outputPath, err := engine.Execute(context.Background(), filepath.Join(dir, "source.docx"), dir, "html")
	if err != nil {
		t.Fatalf("docx to html: %v", err)
	}
	htmlData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(htmlData)), "<html") {
		t.Fatalf("expected html document output")
	}
}

func TestToDocxEngineConvertsLegacyDocFixture(t *testing.T) {
	requireRuntime(t, "libreoffice")

	dir := t.TempDir()
	outputPath, err := (&ToDocxEngine{}).Execute(
		context.Background(),
		documentFixturePath("doc", "valid-basic.doc"),
		dir,
		"docx",
	)
	if err != nil {
		t.Fatalf("legacy doc to docx: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read docx: %v", err)
	}
	if !strings.HasPrefix(string(data[:2]), "PK") {
		t.Fatalf("expected docx zip header, got %q", data[:2])
	}
}

func TestMarkdownToHTMLEngineRendersHTML(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sample.md")
	if err := os.WriteFile(inputPath, []byte("# Title\n\n- item\n\n[link](https://example.com)\n"), 0o644); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	engine := &MarkdownToHTMLEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "html")
	if err != nil {
		t.Fatalf("markdown to html: %v", err)
	}
	htmlData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	htmlText := string(htmlData)
	if !strings.Contains(htmlText, "<h1") || !strings.Contains(htmlText, "<ul>") {
		t.Fatalf("expected rendered heading and list in html output")
	}
}

func TestMarkdownToPDFEngineConvertsRenderedHTML(t *testing.T) {
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sample.md")
	if err := os.WriteFile(inputPath, []byte("# Title\n\nParagraph.\n"), 0o644); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	engine := &MarkdownToPDFEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "pdf")
	if err != nil {
		t.Fatalf("markdown to pdf: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if !strings.HasPrefix(string(data[:5]), "%PDF-") {
		t.Fatalf("expected pdf header, got %q", data[:5])
	}
}

func TestToPDFEngineConvertsHTMLFile(t *testing.T) {
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sample.html")
	html := "<!doctype html><html><body><h1>Hola</h1><p>Preview</p></body></html>"
	if err := os.WriteFile(inputPath, []byte(html), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	engine := &ToPDFEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "pdf")
	if err != nil {
		t.Fatalf("html to pdf: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if !strings.HasPrefix(string(data[:5]), "%PDF-") {
		t.Fatalf("expected pdf header, got %q", data[:5])
	}
}

func TestHTMLToTextEngineExtractsReadableText(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sample.html")
	html := "<!doctype html><html><body><h1>Hola</h1><p>Texto base</p><ul><li>Uno</li><li>Dos</li></ul><script>window.secret = 'ignored';</script></body></html>"
	if err := os.WriteFile(inputPath, []byte(html), 0o644); err != nil {
		t.Fatalf("write html: %v", err)
	}

	outputPath, err := (&HTMLToTextEngine{}).Execute(context.Background(), inputPath, dir, "txt")
	if err != nil {
		t.Fatalf("html to txt: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read txt: %v", err)
	}
	text := string(data)
	for _, expected := range []string{"Hola", "Texto base", "- Uno", "- Dos"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in extracted text, got %q", expected, text)
		}
	}
	if strings.Contains(text, "window.secret") {
		t.Fatalf("expected script contents to be ignored, got %q", text)
	}
}

func TestDOCXToMarkdownEngineConvertsDocument(t *testing.T) {
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(sourcePath, []byte("Titulo\n\ntexto base\n"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	cmd := exec.Command("libreoffice", "--headless", "--convert-to", "docx", "--outdir", dir, sourcePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare docx: %s: %v", strings.TrimSpace(string(out)), err)
	}

	engine := &DOCXToMarkdownEngine{}
	outputPath, err := engine.Execute(context.Background(), filepath.Join(dir, "source.docx"), dir, "md")
	if err != nil {
		t.Fatalf("docx to markdown: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	markdown := string(data)
	if !strings.Contains(markdown, "Titulo") || !strings.Contains(strings.ToLower(markdown), "texto base") {
		t.Fatalf("expected markdown output to preserve document text, got %q", markdown)
	}
}

func TestPresentationToImagesEngineCreatesZIPForComplexDeck(t *testing.T) {
	requireRuntime(t, "libreoffice", "pdftoppm")

	dir := t.TempDir()
	outputPath, err := (&PresentationToImagesEngine{}).Execute(
		context.Background(),
		documentFixturePath("presentation", "valid-three-slides.pptx"),
		dir,
		"jpg",
	)
	if err != nil {
		t.Fatalf("complex presentation to jpg zip: %v", err)
	}
	if filepath.Base(outputPath) != "slides.zip" {
		t.Fatalf("expected slides.zip output, got %s", filepath.Base(outputPath))
	}
	entries := readZipEntries(t, outputPath)
	if len(entries) != 3 {
		t.Fatalf("expected three slide images, got %d", len(entries))
	}
	for _, entry := range entries {
		if !strings.HasSuffix(strings.ToLower(entry), ".jpg") {
			t.Fatalf("expected jpg slide image, got %s", entry)
		}
	}
}

func TestPresentationToImagesEngineRejectsCorruptedDeck(t *testing.T) {
	requireRuntime(t, "libreoffice", "pdftoppm")

	dir := t.TempDir()
	if _, err := (&PresentationToImagesEngine{}).Execute(
		context.Background(),
		documentFixturePath("presentation", "corrupted-invalid.pptx"),
		dir,
		"jpg",
	); err == nil {
		t.Fatal("expected corrupted presentation fixture to fail conversion")
	}
}

func TestMarkdownToDocxEngineConvertsRenderedHTML(t *testing.T) {
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sample.md")
	if err := os.WriteFile(inputPath, []byte("# Title\n\nParagraph.\n"), 0o644); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	engine := &MarkdownToDocxEngine{}
	outputPath, err := engine.Execute(context.Background(), inputPath, dir, "docx")
	if err != nil {
		t.Fatalf("markdown to docx: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read docx: %v", err)
	}
	if !strings.HasPrefix(string(data[:2]), "PK") {
		t.Fatalf("expected docx zip header, got %q", data[:2])
	}
}

func TestSpreadsheetEnginesConvertWorkbook(t *testing.T) {
	if _, err := exec.LookPath("libreoffice"); err != nil {
		t.Skip("libreoffice not available")
	}

	dir := t.TempDir()
	inputPath := filepath.Join(dir, "sheet.csv")
	if err := os.WriteFile(inputPath, []byte("col_a,col_b\n1,2\n3,4\n"), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	xlsxPath, err := (&ToXLSXEngine{}).Execute(context.Background(), inputPath, dir, "xlsx")
	if err != nil {
		t.Skipf("spreadsheet filters unavailable in current runtime: %v", err)
	}
	xlsxData, err := os.ReadFile(xlsxPath)
	if err != nil {
		t.Fatalf("read xlsx: %v", err)
	}
	if !strings.HasPrefix(string(xlsxData[:2]), "PK") {
		t.Fatalf("expected xlsx zip header, got %q", xlsxData[:2])
	}

	htmlPath, err := (&ToHTMLEngine{}).Execute(context.Background(), xlsxPath, dir, "html")
	if err != nil {
		t.Fatalf("xlsx to html: %v", err)
	}
	htmlData, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(htmlData)), "<html") {
		t.Fatalf("expected html document output for spreadsheet")
	}

	pdfPath, err := (&ToPDFEngine{}).Execute(context.Background(), xlsxPath, dir, "pdf")
	if err != nil {
		t.Fatalf("xlsx to pdf: %v", err)
	}
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatalf("read pdf: %v", err)
	}
	if !strings.HasPrefix(string(pdfData[:5]), "%PDF-") {
		t.Fatalf("expected pdf header, got %q", pdfData[:5])
	}

	roundtripCSVPath, err := (&ToCSVEngine{}).Execute(context.Background(), xlsxPath, dir, "csv")
	if err != nil {
		t.Fatalf("xlsx to csv: %v", err)
	}
	roundtripCSV, err := os.ReadFile(roundtripCSVPath)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if !strings.Contains(string(roundtripCSV), "col_a") || !strings.Contains(string(roundtripCSV), "1") {
		t.Fatalf("expected csv export to preserve sheet contents, got %q", roundtripCSV)
	}
}

func TestToCSVEngineConvertsComplexWorkbook(t *testing.T) {
	requireRuntime(t, "libreoffice")

	dir := t.TempDir()
	outputPath, err := (&ToCSVEngine{}).Execute(
		context.Background(),
		documentFixturePath("spreadsheet", "valid-multi-sheet.xlsx"),
		dir,
		"csv",
	)
	if err != nil {
		t.Fatalf("multi-sheet xlsx to csv: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	text := string(data)
	for _, expected := range []string{"capability,status,count", "presentation-to-jpg", "image-heic-to-png"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %q in csv output, got %q", expected, text)
		}
	}
}

func TestToCSVEngineRejectsCorruptedWorkbook(t *testing.T) {
	requireRuntime(t, "libreoffice")

	dir := t.TempDir()
	if _, err := (&ToCSVEngine{}).Execute(
		context.Background(),
		documentFixturePath("spreadsheet", "corrupted-invalid.xlsx"),
		dir,
		"csv",
	); err == nil {
		t.Fatal("expected corrupted spreadsheet fixture to fail conversion")
	}
}
