//go:build !cgo

package document

import (
	"strings"
	"testing"
)

func TestConvertHTMLToMarkdownIgnoresScriptAndStyle(t *testing.T) {
	markdown, err := convertHTMLToMarkdown(`<!doctype html><html><body><h1>Hola</h1><p>Texto base</p><script>window.secret='x'</script><style>body{color:red}</style></body></html>`)
	if err != nil {
		t.Fatalf("convert html: %v", err)
	}
	if strings.Contains(markdown, "window.secret") {
		t.Fatalf("expected script content to be ignored, got %q", markdown)
	}
	if strings.Contains(markdown, "color:red") {
		t.Fatalf("expected style content to be ignored, got %q", markdown)
	}
	if !strings.Contains(markdown, "# Hola") || !strings.Contains(markdown, "Texto base") {
		t.Fatalf("expected visible content to remain, got %q", markdown)
	}
}

func TestConvertHTMLToMarkdownPreservesInlineSpacing(t *testing.T) {
	markdown, err := convertHTMLToMarkdown(`<!doctype html><html><body><p>Texto <strong>base</strong> y <em>enfasis</em>.</p></body></html>`)
	if err != nil {
		t.Fatalf("convert html: %v", err)
	}
	if !strings.Contains(markdown, "Texto **base** y *enfasis*.") {
		t.Fatalf("expected inline markdown spacing to be preserved, got %q", markdown)
	}
}
