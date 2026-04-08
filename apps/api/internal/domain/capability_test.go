package domain

import "testing"

func TestCapability_IsSourceSupported(t *testing.T) {
	cap := Capability{
		SourceFormats: []string{"application/pdf", "image/png"},
	}

	if !cap.IsSourceSupported("application/pdf") {
		t.Error("expected pdf to be supported")
	}
	if !cap.IsSourceSupported("image/png") {
		t.Error("expected png to be supported")
	}
	if cap.IsSourceSupported("text/plain") {
		t.Error("expected text/plain to not be supported")
	}
	if cap.IsSourceSupported("") {
		t.Error("expected empty string to not be supported")
	}
}
