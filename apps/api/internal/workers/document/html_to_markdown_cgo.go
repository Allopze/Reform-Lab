//go:build cgo

package document

import "github.com/kreuzberg-dev/html-to-markdown/packages/go/v2/htmltomarkdown"

func convertHTMLToMarkdown(source string) (string, error) {
	return htmltomarkdown.Convert(source)
}
