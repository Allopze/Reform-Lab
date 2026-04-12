package document

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
)

var (
	// scriptRe removes <script> blocks entirely.
	scriptRe = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)

	// metaRefreshRe removes <meta http-equiv="refresh"> redirects.
	metaRefreshRe = regexp.MustCompile(`(?i)<meta[^>]+http-equiv\s*=\s*["']refresh["'][^>]*>`)

	// remoteAttrRe neutralizes src/data/poster/background attributes with remote URLs.
	remoteAttrRe = regexp.MustCompile(`(?i)((?:src|data|poster|background)\s*=\s*["'])(?:https?://|file://|//)[^"']*?(["'])`)

	// remoteLinkHrefRe neutralizes href on <link> elements with remote URLs.
	remoteLinkHrefRe = regexp.MustCompile(`(?i)(<link[^>]*href\s*=\s*["'])(?:https?://|file://|//)[^"']*?(["'])`)

	// remoteCSSURLRe neutralizes CSS url() with remote URLs.
	remoteCSSURLRe = regexp.MustCompile(`(?i)(url\s*\(\s*["']?)(?:https?://|file://|//)[^)"']*?(["']?\s*\))`)

	// importRe removes CSS @import with remote URLs.
	importRe = regexp.MustCompile(`(?i)@import\s+(?:url\s*\()?["']?(?:https?://|file://|//)[^"');\s]*["']?\)?\s*;?`)
)

// sanitizeHTMLBytes removes remote resource references from HTML content
// to prevent SSRF when processed by conversion engines like LibreOffice.
func sanitizeHTMLBytes(content []byte) []byte {
	result := scriptRe.ReplaceAll(content, nil)
	result = metaRefreshRe.ReplaceAll(result, nil)
	result = remoteAttrRe.ReplaceAll(result, []byte("${1}about:blank${2}"))
	result = remoteLinkHrefRe.ReplaceAll(result, []byte("${1}about:blank${2}"))
	result = remoteCSSURLRe.ReplaceAll(result, []byte("${1}about:blank${2}"))
	result = importRe.ReplaceAll(result, nil)
	return result
}

// SanitizeHTMLBytes is the exported version of sanitizeHTMLBytes for cross-package use.
func SanitizeHTMLBytes(content []byte) []byte {
	return sanitizeHTMLBytes(content)
}

// copyAndSanitizeHTML reads an HTML file, sanitizes remote references, and writes to destPath.
func copyAndSanitizeHTML(srcPath, destPath string) error {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read html: %w", err)
	}
	sanitized := sanitizeHTMLBytes(content)
	if err := os.WriteFile(destPath, sanitized, 0o644); err != nil {
		return fmt.Errorf("write sanitized html: %w", err)
	}
	return nil
}

// looksLikeHTML detects if a file's content is HTML by sniffing the first bytes.
func looksLikeHTML(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n == 0 {
		return false
	}

	contentType := http.DetectContentType(buf[:n])
	return contentType == "text/html; charset=utf-8"
}
