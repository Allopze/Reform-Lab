package webhook

import (
	"context"
	"net/netip"
	"testing"
)

func TestValidateConfiguredURLRejectsLocalhostAndPrivateLiterals(t *testing.T) {
	t.Parallel()

	tests := []string{
		"http://localhost:8080/hook",
		"http://admin:pass@example.com/hook",
		"https://127.0.0.1/hook",
		"https://[::1]/hook",
		"https://10.0.0.5/hook",
	}

	for _, raw := range tests {
		if err := ValidateConfiguredURL(raw); err == nil {
			t.Fatalf("expected configured URL %q to be rejected", raw)
		}
	}
}

func TestValidateConfiguredURLAcceptsPublicURL(t *testing.T) {
	t.Parallel()

	if err := ValidateConfiguredURL("https://hooks.example.com/reform"); err != nil {
		t.Fatalf("expected public URL to be accepted, got %v", err)
	}
}

func TestValidateDeliveryTargetRejectsPrivateResolution(t *testing.T) {
	t.Parallel()

	err := validateDeliveryTargetWithLookup(context.Background(), "https://hooks.example.com/reform", func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("10.1.2.3")}, nil
	})
	if err == nil {
		t.Fatal("expected private resolved address to be rejected")
	}
}

func TestValidateDeliveryTargetAcceptsPublicResolution(t *testing.T) {
	t.Parallel()

	err := validateDeliveryTargetWithLookup(context.Background(), "https://hooks.example.com/reform", func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	})
	if err != nil {
		t.Fatalf("expected public resolved address to be accepted, got %v", err)
	}
}
