package document

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	// remoteCSSURLRe neutralizes CSS url() with remote URLs.
	remoteCSSURLRe = regexp.MustCompile(`(?i)(url\s*\(\s*["']?)(?:https?://|file://|//)[^)"']*?(["']?\s*\))`)

	// importRe removes CSS @import with remote URLs.
	importRe = regexp.MustCompile(`(?i)@import\s+(?:url\s*\()?["']?(?:https?://|file://|//)[^"');\s]*["']?\)?\s*;?`)
)

// sanitizeHTMLBytes removes remote resource references from HTML content
// to prevent SSRF when processed by conversion engines like LibreOffice.
func sanitizeHTMLBytes(content []byte) []byte {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return sanitizeCSSBytes(content)
	}

	sanitizeHTMLNode(doc)

	var sanitized bytes.Buffer
	if err := html.Render(&sanitized, doc); err != nil {
		return sanitizeCSSBytes(content)
	}
	return sanitized.Bytes()
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

func sanitizeHTMLNode(node *html.Node) {
	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		sanitizeHTMLNode(child)
		child = next
	}

	if node.Type != html.ElementNode {
		return
	}

	tagName := strings.ToLower(node.Data)
	if shouldRemoveElement(node, tagName) {
		removeNode(node)
		return
	}

	sanitizeNodeAttributes(node, tagName)

	if tagName == "style" && node.FirstChild != nil && node.FirstChild.Type == html.TextNode {
		node.FirstChild.Data = string(sanitizeCSSBytes([]byte(node.FirstChild.Data)))
	}
}

func shouldRemoveElement(node *html.Node, tagName string) bool {
	switch tagName {
	case "script", "iframe", "object", "embed", "base", "foreignobject":
		return true
	case "meta":
		return strings.EqualFold(getAttrValue(node, "http-equiv"), "refresh")
	default:
		return false
	}
}

func sanitizeNodeAttributes(node *html.Node, tagName string) {
	attrs := make([]html.Attribute, 0, len(node.Attr))
	for _, attr := range node.Attr {
		attrName := attr.Key
		if attr.Namespace != "" {
			attrName = attr.Namespace + ":" + attr.Key
		}

		if strings.HasPrefix(strings.ToLower(attrName), "on") {
			continue
		}

		switch {
		case shouldNeutralizeResourceAttr(tagName, attrName):
			attr.Val = sanitizeResourceAttribute(attr.Val)
		case shouldNeutralizeDangerousURLAttr(tagName, attrName):
			attr.Val = sanitizeNavigationalAttribute(attr.Val)
		case strings.EqualFold(attrName, "style"):
			attr.Val = string(sanitizeCSSBytes([]byte(attr.Val)))
		}

		attrs = append(attrs, attr)
	}
	node.Attr = attrs
}

func shouldNeutralizeResourceAttr(tagName, attrName string) bool {
	switch strings.ToLower(attrName) {
	case "src", "data", "poster", "background":
		return true
	case "srcset":
		return true
	case "href":
		switch tagName {
		case "link", "image", "use", "feimage":
			return true
		}
	case "xlink:href":
		switch tagName {
		case "image", "use", "feimage":
			return true
		}
	}
	return false
}

func shouldNeutralizeDangerousURLAttr(tagName, attrName string) bool {
	switch strings.ToLower(attrName) {
	case "href":
		return tagName == "a" || tagName == "area"
	case "action", "formaction":
		return true
	default:
		return false
	}
}

func sanitizeResourceAttribute(value string) string {
	if isDangerousURLScheme(value) || isRemoteResourceURL(value) {
		return "about:blank"
	}
	return sanitizeSrcset(value)
}

func sanitizeNavigationalAttribute(value string) string {
	if isDangerousURLScheme(value) {
		return "about:blank"
	}
	return value
}

func isRemoteResourceURL(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(trimmed, "http://") ||
		strings.HasPrefix(trimmed, "https://") ||
		strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "file://")
}

func isDangerousURLScheme(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(trimmed, "javascript:") ||
		strings.HasPrefix(trimmed, "vbscript:") ||
		strings.HasPrefix(trimmed, "data:text/html") ||
		strings.HasPrefix(trimmed, "data:application/javascript")
}

func sanitizeSrcset(value string) string {
	if !strings.Contains(value, ",") {
		return value
	}
	candidates := strings.Split(value, ",")
	for i, candidate := range candidates {
		parts := strings.Fields(strings.TrimSpace(candidate))
		if len(parts) == 0 {
			continue
		}
		if isRemoteResourceURL(parts[0]) || isDangerousURLScheme(parts[0]) {
			parts[0] = "about:blank"
		}
		candidates[i] = strings.Join(parts, " ")
	}
	return strings.Join(candidates, ", ")
}

func sanitizeCSSBytes(content []byte) []byte {
	result := remoteCSSURLRe.ReplaceAll(content, []byte("${1}about:blank${2}"))
	return importRe.ReplaceAll(result, nil)
}

func getAttrValue(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) && attr.Namespace == "" {
			return strings.TrimSpace(attr.Val)
		}
	}
	return ""
}

func removeNode(node *html.Node) {
	if node.Parent == nil {
		return
	}
	node.Parent.RemoveChild(node)
}

// looksLikeHTML detects if a file's content is HTML by sniffing the first bytes.
func looksLikeHTML(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	tokenizer := html.NewTokenizer(io.LimitReader(f, 4096))
	for {
		tokenType := tokenizer.Next()
		switch tokenType {
		case html.ErrorToken:
			return false
		case html.StartTagToken:
			return true
		case html.TextToken:
			if strings.TrimSpace(string(tokenizer.Text())) != "" {
				return false
			}
		case html.CommentToken, html.DoctypeToken:
			continue
		}
	}
}
