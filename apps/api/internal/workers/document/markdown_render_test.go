package document

import (
	"strings"
	"testing"
)

func TestRenderMarkdownDocumentEscapesRawHTML(t *testing.T) {
	output, err := renderMarkdownDocument([]byte("# Title\n\n<script>alert('xss')</script>\n"))
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	html := string(output)
	if strings.Contains(html, "<script>alert('xss')</script>") {
		t.Fatal("expected raw script tag to be escaped")
	}
	if !strings.Contains(html, "<!-- raw HTML omitted -->") {
		t.Fatalf("expected raw HTML to be omitted safely, got %q", html)
	}
}
