package document

import (
	"bytes"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	rendererhtml "github.com/yuin/goldmark/renderer/html"
)

var markdownRenderer = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Linkify,
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(
		rendererhtml.WithHardWraps(),
	),
)

func renderMarkdownDocument(source []byte) ([]byte, error) {
	var body bytes.Buffer
	if err := markdownRenderer.Convert(source, &body); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}

	var doc bytes.Buffer
	doc.WriteString("<!doctype html>\n")
	doc.WriteString("<html lang=\"en\">\n<head>\n")
	doc.WriteString("<meta charset=\"utf-8\">\n")
	doc.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	doc.WriteString("<style>body{font-family:Georgia,serif;line-height:1.6;margin:32px;color:#1f2937;}pre,code{font-family:monospace;}pre{background:#f3f4f6;padding:12px;overflow:auto;}blockquote{border-left:4px solid #d1d5db;padding-left:12px;color:#4b5563;}table{border-collapse:collapse;}th,td{border:1px solid #d1d5db;padding:6px 10px;}img{max-width:100%;height:auto;}</style>\n")
	doc.WriteString("</head>\n<body>\n")
	doc.Write(body.Bytes())
	doc.WriteString("\n</body>\n</html>\n")

	return doc.Bytes(), nil
}
