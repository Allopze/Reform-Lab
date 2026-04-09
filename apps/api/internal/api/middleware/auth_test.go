package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractTokenUsesSessionCookieOnly(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "cookie-session-token"})
	req.Header.Set("Authorization", "Bearer header-token")

	if got := extractToken(req); got != "cookie-session-token" {
		t.Fatalf("expected cookie token, got %q", got)
	}
}

func TestExtractTokenIgnoresAuthorizationHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer header-token")

	if got := extractToken(req); got != "" {
		t.Fatalf("expected Authorization header to be ignored, got %q", got)
	}
}
