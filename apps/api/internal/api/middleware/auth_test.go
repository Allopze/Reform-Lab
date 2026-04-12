package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsAnyConfiguredOriginFromList(t *testing.T) {
	handler := CORS("http://localhost:5050, http://192.168.4.111:5050")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/auth/register", nil)
	req.Header.Set("Origin", "http://192.168.4.111:5050")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://192.168.4.111:5050" {
		t.Fatalf("expected network origin to be allowed, got %q", got)
	}
}

func TestCORSAllowsPutPreflight(t *testing.T) {
	handler := CORS("http://localhost:5050")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/admin/footer-message", nil)
	req.Header.Set("Origin", "http://localhost:5050")
	req.Header.Set("Access-Control-Request-Method", http.MethodPut)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5050" {
		t.Fatalf("expected localhost origin to be allowed, got %q", got)
	}
	if got := res.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPut) {
		t.Fatalf("expected preflight methods to include PUT, got %q", got)
	}
}

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
