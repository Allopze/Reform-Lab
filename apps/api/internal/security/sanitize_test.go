package security

import (
	"strings"
	"testing"
)

func TestSanitizeFileName_StripsPathComponents(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/etc/passwd", "passwd"},
		// On Linux, filepath.Base doesn't recognize backslashes as separators.
		// Backslashes are stripped by strings.Map, leaving the rest intact.
		{"C:\\Windows\\System32\\config", "C:WindowsSystem32config"},
		{"../../../etc/shadow", "shadow"},
		{"foo/bar/baz.txt", "baz.txt"},
		{"/tmp/test.pdf", "test.pdf"},
	}
	for _, tc := range tests {
		if got := SanitizeFileName(tc.input); got != tc.expected {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSanitizeFileName_PathTraversal(t *testing.T) {
	// Path traversal attempts should be neutralized
	traversal := SanitizeFileName("../../etc/passwd")
	if strings.Contains(traversal, "..") {
		t.Fatalf("SanitizeFileName should strip path traversal, got %q", traversal)
	}

	traversal2 := SanitizeFileName("foo/../../bar")
	if strings.Contains(traversal2, "..") {
		t.Fatalf("SanitizeFileName should strip path traversal, got %q", traversal2)
	}
}

func TestSanitizeFileName_NullBytes(t *testing.T) {
	// Null bytes should be stripped
	input := "file\x00.txt"
	got := SanitizeFileName(input)
	if strings.Contains(got, "\x00") {
		t.Fatalf("SanitizeFileName should strip null bytes, got %q", got)
	}
}

func TestSanitizeFileName_UnicodeNames(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"documento.pdf", "documento.pdf"},
		{"文件.txt", "文件.txt"},
		{"файл.docx", "файл.docx"},
		{"αβγ.md", "αβγ.md"},
		{"日本語プレゼン.pptx", "日本語プレゼン.pptx"},
	}
	for _, tc := range tests {
		if got := SanitizeFileName(tc.input); got != tc.expected {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSanitizeFileName_EmptyAndSpecial(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "unnamed"},
		{".", "unnamed"},
		{"..", "unnamed"},
		{"   ", "   "}, // spaces are printable
	}
	for _, tc := range tests {
		if got := SanitizeFileName(tc.input); got != tc.expected {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSanitizeFileName_LimitsLength(t *testing.T) {
	longName := strings.Repeat("a", 300) + ".pdf"
	got := SanitizeFileName(longName)
	if len(got) > 255 {
		t.Fatalf("SanitizeFileName should limit to 255 chars, got %d", len(got))
	}
}

func TestSanitizeFileName_StripsNonPrintable(t *testing.T) {
	// Control characters should be stripped
	input := "file\x01\x02\x03name.txt"
	got := SanitizeFileName(input)
	if strings.ContainsAny(got, "\x01\x02\x03") {
		t.Fatalf("SanitizeFileName should strip control characters, got %q", got)
	}
}

func TestSanitizeFileName_PreservesValidExtensions(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"report.pdf", "report.pdf"},
		{"image.PNG", "image.PNG"},
		{"archive.tar.gz", "archive.tar.gz"},
		{"file.with.many.dots.txt", "file.with.many.dots.txt"},
	}
	for _, tc := range tests {
		if got := SanitizeFileName(tc.input); got != tc.expected {
			t.Errorf("SanitizeFileName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestSanitizeFileName_MixedAttack(t *testing.T) {
	// Combined attack: path traversal + null byte + unicode
	input := "../../\x00恶意文件\x00.pdf"
	got := SanitizeFileName(input)
	if strings.Contains(got, "..") || strings.Contains(got, "\x00") {
		t.Fatalf("SanitizeFileName should neutralize mixed attacks, got %q", got)
	}
	// Should still preserve the safe parts
	if got != "恶意文件.pdf" {
		t.Errorf("SanitizeFileName(%q) = %q, want %q", input, got, "恶意文件.pdf")
	}
}
