package security

import (
	"path/filepath"
	"strings"
	"unicode"
)

// SanitizeFileName returns a safe display name, stripping path components
// and problematic characters. Never use for internal storage paths.
func SanitizeFileName(name string) string {
	// Take only the base name (strip directory components).
	name = filepath.Base(name)

	// Replace path separators and null bytes.
	name = strings.Map(func(r rune) rune {
		if r == 0 || r == '/' || r == '\\' {
			return -1
		}
		if !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, name)

	if name == "" || name == "." || name == ".." {
		return "unnamed"
	}

	// Limit length.
	if len(name) > 255 {
		name = name[:255]
	}

	return name
}
