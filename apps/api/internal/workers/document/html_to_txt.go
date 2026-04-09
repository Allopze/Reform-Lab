package document

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

// HTMLToTextEngine extracts readable text blocks from HTML without preserving markup.
type HTMLToTextEngine struct{}

func (e *HTMLToTextEngine) Execute(_ context.Context, inputPath, outputDir, _ string) (string, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return "", fmt.Errorf("read html: %w", err)
	}

	doc, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	var builder strings.Builder
	extractHTMLText(&builder, doc)
	text := normalizeExtractedText(builder.String())
	if text == "" {
		return "", fmt.Errorf("html contains no extractable text")
	}

	outputPath := filepath.Join(outputDir, "extracted.txt")
	if err := os.WriteFile(outputPath, []byte(text+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write text output: %w", err)
	}
	return outputPath, nil
}

var htmlBlockElements = map[string]bool{
	"address":    true,
	"article":    true,
	"aside":      true,
	"blockquote": true,
	"div":        true,
	"dl":         true,
	"dt":         true,
	"dd":         true,
	"fieldset":   true,
	"figcaption": true,
	"figure":     true,
	"footer":     true,
	"form":       true,
	"h1":         true,
	"h2":         true,
	"h3":         true,
	"h4":         true,
	"h5":         true,
	"h6":         true,
	"header":     true,
	"li":         true,
	"main":       true,
	"nav":        true,
	"ol":         true,
	"p":          true,
	"pre":        true,
	"section":    true,
	"table":      true,
	"tr":         true,
	"td":         true,
	"th":         true,
	"ul":         true,
}

var htmlIgnoredElements = map[string]bool{
	"noscript": true,
	"script":   true,
	"style":    true,
}

func extractHTMLText(builder *strings.Builder, node *html.Node) {
	if node == nil {
		return
	}

	if node.Type == html.ElementNode {
		if htmlIgnoredElements[node.Data] {
			return
		}
		if node.Data == "br" {
			builder.WriteString("\n")
			return
		}
		if node.Data == "li" {
			if !endsWithLineBreak(builder) {
				builder.WriteString("\n")
			}
			builder.WriteString("- ")
		}
	}

	if node.Type == html.TextNode {
		writeNormalizedHTMLText(builder, node.Data)
	}

	for child := node.FirstChild; child != nil; child = child.NextSibling {
		extractHTMLText(builder, child)
	}

	if node.Type == html.ElementNode && htmlBlockElements[node.Data] && !endsWithLineBreak(builder) {
		builder.WriteString("\n")
	}
}

func writeNormalizedHTMLText(builder *strings.Builder, value string) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return
	}
	if builder.Len() > 0 && !endsWithSpace(builder) && !endsWithLineBreak(builder) {
		builder.WriteByte(' ')
	}
	builder.WriteString(strings.Join(parts, " "))
}

func normalizeExtractedText(value string) string {
	rawLines := strings.Split(value, "\n")
	lines := make([]string, 0, len(rawLines))
	blankPending := false
	for _, line := range rawLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !blankPending && len(lines) > 0 {
				lines = append(lines, "")
				blankPending = true
			}
			continue
		}
		lines = append(lines, trimmed)
		blankPending = false
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func endsWithSpace(builder *strings.Builder) bool {
	value := builder.String()
	if value == "" {
		return false
	}
	last := value[len(value)-1]
	return last == ' ' || last == '\t'
}

func endsWithLineBreak(builder *strings.Builder) bool {
	value := builder.String()
	return value == "" || value[len(value)-1] == '\n'
}
