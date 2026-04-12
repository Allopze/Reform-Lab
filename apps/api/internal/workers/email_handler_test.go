package workers

import "testing"

func TestEmailDomain(t *testing.T) {
	tests := map[string]string{
		"user@example.com":   "example.com",
		" User@Example.COM ": "example.com",
		"invalid-address":    "unknown",
		"":                   "unknown",
	}

	for input, want := range tests {
		if got := emailDomain(input); got != want {
			t.Fatalf("emailDomain(%q) = %q, want %q", input, got, want)
		}
	}
}
