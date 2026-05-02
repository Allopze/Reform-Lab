package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeHTMLBytes_RemovesScripts(t *testing.T) {
	input := []byte(`<html><body><script>alert("xss")</script><p>Hello</p></body></html>`)
	result := sanitizeHTMLBytes(input)
	if strings.Contains(string(result), "<script") {
		t.Error("expected script tags to be removed")
	}
	if !strings.Contains(string(result), "<p>Hello</p>") {
		t.Error("expected body content to be preserved")
	}
}

func TestSanitizeHTMLBytes_NeutralizesRemoteSrc(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"http img", `<img src="http://evil.com/pixel.png">`},
		{"https img", `<img src="https://evil.com/pixel.png">`},
		{"file img", `<img src="file:///etc/passwd">`},
		{"protocol-relative", `<img src="//evil.com/pixel.png">`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeHTMLBytes([]byte(tt.input))
			s := string(result)
			if strings.Contains(s, "evil.com") || strings.Contains(s, "/etc/passwd") {
				t.Errorf("expected remote URL to be neutralized, got: %s", s)
			}
			if !strings.Contains(s, "about:blank") {
				t.Errorf("expected about:blank replacement, got: %s", s)
			}
		})
	}
}

func TestSanitizeHTMLBytes_NeutralizesLinkHref(t *testing.T) {
	input := []byte(`<link rel="stylesheet" href="https://evil.com/style.css">`)
	result := sanitizeHTMLBytes(input)
	if strings.Contains(string(result), "evil.com") {
		t.Error("expected remote link href to be neutralized")
	}
}

func TestSanitizeHTMLBytes_NeutralizesCSSUrl(t *testing.T) {
	input := []byte(`<div style="background: url('https://evil.com/bg.png')">`)
	result := sanitizeHTMLBytes(input)
	if strings.Contains(string(result), "evil.com") {
		t.Error("expected remote CSS url to be neutralized")
	}
}

func TestSanitizeHTMLBytes_RemovesImportRemote(t *testing.T) {
	input := []byte(`<style>@import url("https://evil.com/style.css");</style>`)
	result := sanitizeHTMLBytes(input)
	if strings.Contains(string(result), "evil.com") {
		t.Error("expected @import with remote URL to be removed")
	}
}

func TestSanitizeHTMLBytes_RemovesMetaRefresh(t *testing.T) {
	input := []byte(`<meta http-equiv="refresh" content="0;url=http://evil.com">`)
	result := sanitizeHTMLBytes(input)
	if strings.Contains(string(result), "evil.com") {
		t.Error("expected meta refresh to be removed")
	}
}

func TestSanitizeHTMLBytes_PreservesLocalContent(t *testing.T) {
	input := []byte(`<html><body><h1>Title</h1><p>Content</p><a href="https://example.com">link</a></body></html>`)
	result := sanitizeHTMLBytes(input)
	s := string(result)
	if !strings.Contains(s, "<h1>Title</h1>") {
		t.Error("expected body content to be preserved")
	}
	// <a href> should not be neutralized (anchors don't cause fetches during rendering)
	if !strings.Contains(s, `href="https://example.com"`) {
		t.Error("expected anchor href to be preserved")
	}
}

func TestSanitizeHTMLBytes_PreservesInlineImages(t *testing.T) {
	input := []byte(`<img src="data:image/png;base64,iVBOR...">`)
	result := sanitizeHTMLBytes(input)
	if !strings.Contains(string(result), "data:image/png") {
		t.Error("expected data URI images to be preserved")
	}
}

func TestSanitizeHTMLBytes_NeutralizesEntityEncodedRemoteSrc(t *testing.T) {
	input := []byte(`<img src="&#x68;ttp://evil.com/pixel.png">`)
	result := sanitizeHTMLBytes(input)
	s := string(result)
	if strings.Contains(s, "evil.com") {
		t.Fatalf("expected entity-encoded remote URL to be neutralized, got %s", s)
	}
	if !strings.Contains(s, "about:blank") {
		t.Fatalf("expected about:blank replacement, got %s", s)
	}
}

func TestSanitizeHTMLBytes_RemovesEventHandlers(t *testing.T) {
	input := []byte(`<img src="/safe.png" onload="alert('xss')"><p onclick="steal()">Hi</p>`)
	result := sanitizeHTMLBytes(input)
	s := string(result)
	if strings.Contains(s, "onload=") || strings.Contains(s, "onclick=") {
		t.Fatalf("expected event handlers to be removed, got %s", s)
	}
	if !strings.Contains(s, `/safe.png`) {
		t.Fatalf("expected safe local resource to remain, got %s", s)
	}
}

func TestSanitizeHTMLBytes_NeutralizesRemoteSVGHref(t *testing.T) {
	input := []byte(`<svg><use xlink:href="https://evil.com/icons.svg#shape"></use></svg>`)
	result := sanitizeHTMLBytes(input)
	s := string(result)
	if strings.Contains(s, "evil.com") {
		t.Fatalf("expected remote SVG href to be neutralized, got %s", s)
	}
	if !strings.Contains(s, "about:blank") {
		t.Fatalf("expected about:blank replacement, got %s", s)
	}
}

func TestSanitizeHTMLBytes_NeutralizesJavascriptHref(t *testing.T) {
	input := []byte(`<a href="javascript:alert('xss')">click</a>`)
	result := sanitizeHTMLBytes(input)
	s := string(result)
	if strings.Contains(strings.ToLower(s), "javascript:") {
		t.Fatalf("expected javascript href to be neutralized, got %s", s)
	}
	if !strings.Contains(s, `href="about:blank"`) {
		t.Fatalf("expected about:blank replacement, got %s", s)
	}
}

func TestSanitizeHTMLBytes_NeutralizesMixedSrcset(t *testing.T) {
	input := []byte(`<img srcset="/safe.png 1x, https://evil.com/evil.png 2x">`)
	result := sanitizeHTMLBytes(input)
	s := string(result)
	if strings.Contains(s, "evil.com") {
		t.Fatalf("expected remote srcset candidate to be neutralized, got %s", s)
	}
	if !strings.Contains(s, `/safe.png 1x, about:blank 2x`) {
		t.Fatalf("expected local srcset candidate to remain and remote one to be neutralized, got %s", s)
	}
}

func TestSanitizeHTMLBytes_RemovesForeignObject(t *testing.T) {
	input := []byte(`<svg><foreignObject><body><script>alert(1)</script><p>Hola</p></body></foreignObject></svg>`)
	result := sanitizeHTMLBytes(input)
	s := string(result)
	if strings.Contains(strings.ToLower(s), "foreignobject") {
		t.Fatalf("expected foreignObject to be removed, got %s", s)
	}
}

func TestLooksLikeHTMLAcceptsLeadingComment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.html")
	if err := os.WriteFile(path, []byte(`<!-- exported by editor --><html><body>Hola</body></html>`), 0o600); err != nil {
		t.Fatalf("write html: %v", err)
	}

	if !looksLikeHTML(path) {
		t.Fatal("expected leading-comment HTML to be recognized")
	}
}

func TestLooksLikeHTMLRejectsPlainText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(`Hola mundo`), 0o600); err != nil {
		t.Fatalf("write text: %v", err)
	}

	if looksLikeHTML(path) {
		t.Fatal("expected plain text not to be recognized as HTML")
	}
}
