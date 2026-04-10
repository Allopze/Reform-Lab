//go:build !cgo

package document

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

func convertHTMLToMarkdown(source string) (string, error) {
	doc, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return "", fmt.Errorf("parse html: %w", err)
	}

	var builder strings.Builder
	renderMarkdownBlock(&builder, doc, 0)
	return normalizeMarkdown(builder.String()), nil
}

func renderMarkdownBlock(builder *strings.Builder, node *html.Node, listDepth int) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.ElementNode:
			if htmlIgnoredElements[child.Data] {
				continue
			}
			switch child.Data {
			case "h1", "h2", "h3", "h4", "h5", "h6":
				level := int(child.Data[1] - '0')
				builder.WriteString(strings.Repeat("#", level))
				builder.WriteByte(' ')
				renderMarkdownInline(builder, child)
				builder.WriteString("\n\n")
			case "p":
				renderMarkdownInline(builder, child)
				builder.WriteString("\n\n")
			case "ul", "ol":
				renderMarkdownBlock(builder, child, listDepth+1)
				builder.WriteByte('\n')
			case "li":
				builder.WriteString(strings.Repeat("  ", max(listDepth-1, 0)))
				builder.WriteString("- ")
				renderMarkdownInline(builder, child)
				builder.WriteByte('\n')
			case "blockquote":
				quoted := extractMarkdownText(child)
				for _, line := range strings.Split(quoted, "\n") {
					trimmed := strings.TrimSpace(line)
					if trimmed == "" {
						continue
					}
					builder.WriteString("> ")
					builder.WriteString(trimmed)
					builder.WriteByte('\n')
				}
				builder.WriteByte('\n')
			default:
				renderMarkdownBlock(builder, child, listDepth)
			}
		case html.TextNode:
			text := strings.TrimSpace(child.Data)
			if text != "" {
				builder.WriteString(text)
				builder.WriteByte('\n')
			}
		}
	}
}

func renderMarkdownInline(builder *strings.Builder, node *html.Node) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			writeMarkdownInlineText(builder, child.Data)
		case html.ElementNode:
			if htmlIgnoredElements[child.Data] {
				continue
			}
			switch child.Data {
			case "strong", "b":
				builder.WriteString("**")
				renderMarkdownInline(builder, child)
				builder.WriteString("**")
			case "em", "i":
				builder.WriteByte('*')
				renderMarkdownInline(builder, child)
				builder.WriteByte('*')
			case "code":
				builder.WriteByte('`')
				renderMarkdownInline(builder, child)
				builder.WriteByte('`')
			case "br":
				builder.WriteString("\n")
			case "a":
				text := strings.TrimSpace(extractMarkdownText(child))
				href := htmlAttr(child, "href")
				if href == "" {
					href = htmlAttr(child, "xlink:href")
				}
				if href != "" {
					if text == "" {
						text = href
					}
					builder.WriteString("[")
					builder.WriteString(text)
					builder.WriteString("](")
					builder.WriteString(href)
					builder.WriteByte(')')
				} else {
					renderMarkdownInline(builder, child)
				}
			default:
				renderMarkdownInline(builder, child)
			}
		}
	}
}

func extractMarkdownText(node *html.Node) string {
	var builder strings.Builder
	renderMarkdownInline(&builder, node)
	return compactWhitespace(builder.String())
}

func writeMarkdownInlineText(builder *strings.Builder, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}

	leadingSpace := hasHTMLBoundarySpace(value[0])
	trailingSpace := hasHTMLBoundarySpace(value[len(value)-1])
	compact := compactWhitespace(value)

	if leadingSpace && !endsWithMarkdownSeparator(builder) {
		builder.WriteByte(' ')
	}
	builder.WriteString(compact)
	if trailingSpace {
		builder.WriteByte(' ')
	}
}

func compactWhitespace(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func hasHTMLBoundarySpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f':
		return true
	default:
		return false
	}
}

func endsWithMarkdownSeparator(builder *strings.Builder) bool {
	value := builder.String()
	if value == "" {
		return true
	}
	last := value[len(value)-1]
	return last == ' ' || last == '\n' || last == '\t'
}

func htmlAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func normalizeMarkdown(value string) string {
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	previousBlank := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if strings.TrimSpace(trimmed) == "" {
			if previousBlank {
				continue
			}
			previousBlank = true
			cleaned = append(cleaned, "")
			continue
		}
		previousBlank = false
		cleaned = append(cleaned, trimmed)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
