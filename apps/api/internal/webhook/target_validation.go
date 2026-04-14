package webhook

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var (
	carrierGradeNATPrefix = netip.MustParsePrefix("100.64.0.0/10")
	benchmarkPrefix       = netip.MustParsePrefix("198.18.0.0/15")
)

type lookupNetIPFunc func(context.Context, string, string) ([]netip.Addr, error)

type errString string

func (e errString) Error() string { return string(e) }

func ValidateConfiguredURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errString("invalid webhook URL")
	}
	return validateParsedWebhookURL(parsed)
}

func ValidateDeliveryTarget(ctx context.Context, raw string) error {
	return validateDeliveryTargetWithLookup(ctx, raw, net.DefaultResolver.LookupNetIP)
}

func NewRestrictedHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil

	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("parse webhook dial address: %w", err)
		}
		if err := validateResolvedHost(ctx, host, net.DefaultResolver.LookupNetIP); err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, address)
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func validateDeliveryTargetWithLookup(ctx context.Context, raw string, lookup lookupNetIPFunc) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errString("invalid webhook URL")
	}
	if err := validateParsedWebhookURL(parsed); err != nil {
		return err
	}
	return validateResolvedHost(ctx, parsed.Hostname(), lookup)
}

func validateParsedWebhookURL(parsed *url.URL) error {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errString("webhook URL must use http or https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return errString("invalid webhook URL")
	}
	if parsed.User != nil {
		return errString("webhook URL must not include credentials")
	}

	host := parsed.Hostname()
	if isDisallowedWebhookHost(host) {
		return errString("webhook URL host is not allowed")
	}
	if addr, err := netip.ParseAddr(host); err == nil && isDisallowedWebhookAddr(addr) {
		return errString("webhook URL host is not allowed")
	}
	return nil
}

func validateResolvedHost(ctx context.Context, host string, lookup lookupNetIPFunc) error {
	if isDisallowedWebhookHost(host) {
		return errString("webhook URL host is not allowed")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if isDisallowedWebhookAddr(addr) {
			return errString("webhook host resolves to a disallowed address")
		}
		return nil
	}

	addrs, err := lookup(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve webhook host: %w", err)
	}
	if len(addrs) == 0 {
		return errString("webhook host resolved to no addresses")
	}
	for _, addr := range addrs {
		if isDisallowedWebhookAddr(addr) {
			return errString("webhook host resolves to a disallowed address")
		}
	}
	return nil
}

func isDisallowedWebhookHost(host string) bool {
	normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return normalized == "localhost" || strings.HasSuffix(normalized, ".localhost")
}

func isDisallowedWebhookAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() {
		return true
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	return carrierGradeNATPrefix.Contains(addr) || benchmarkPrefix.Contains(addr)
}
