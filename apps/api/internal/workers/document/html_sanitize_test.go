package document

import (
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
