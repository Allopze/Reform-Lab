package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

func TestRateLimitAllowsWithinBurst(t *testing.T) {
	handler := RateLimit(10, 5, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First 5 requests from the same IP (burst) should pass.
	for i := 0; i < 5; i++ {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rr.Code)
		}
	}
}

func TestRateLimitRejectsExcess(t *testing.T) {
	handler := RateLimit(1, 1, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request consumes the burst.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rr.Code)
	}

	// Second immediate request from the same IP should be rate limited.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rr.Code)
	}
	if rr.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestRateLimitPerIPIsolation(t *testing.T) {
	handler := RateLimit(1, 1, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Exhaust IP A's bucket.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("IP A first request: expected 200, got %d", rr.Code)
	}

	// IP A should be rejected.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:9999"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("IP A second request: expected 429, got %d", rr.Code)
	}

	// IP B should still be allowed (different IP, separate bucket).
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.2:9999"
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("IP B first request: expected 200, got %d", rr.Code)
	}
}

func TestRateLimitRespectsXForwardedFor(t *testing.T) {
	handler := RateLimit(1, 1, true)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request from proxy with X-Forwarded-For.
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rr.Code)
	}

	// Second request from same real IP should be limited.
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18")
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("second request from same forwarded IP: expected 429, got %d", rr.Code)
	}
}

func TestRateLimitIgnoresForwardedForWhenProxyHeadersDisabled(t *testing.T) {
	handler := RateLimit(1, 1, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.10")
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("expected remote addr to be used when proxy headers are disabled, got %d", second.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	handler.ServeHTTP(rr, req)

	expected := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"X-XSS-Protection":          "0",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
	}
	for header, want := range expected {
		got := rr.Header().Get(header)
		if got != want {
			t.Errorf("header %s: expected %q, got %q", header, want, got)
		}
	}
}

func TestSecurityHeadersSkipsHSTSForPlainHTTP(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rr.Header().Get("Strict-Transport-Security"); got != "" {
		t.Fatalf("expected no HSTS header for plain HTTP, got %q", got)
	}
}

func TestAuthenticatedUserRateLimitIsPerUser(t *testing.T) {
	handler := AuthenticatedUserRateLimit(60, 1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	userA := &domain.User{ID: uuid.New(), Role: domain.RoleUser}
	userB := &domain.User{ID: uuid.New(), Role: domain.RoleUser}

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/conversions", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey{}, userA))
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/conversions", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey{}, userA))
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second request for same user: expected 429, got %d", second.Code)
	}

	third := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/conversions", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey{}, userB))
	handler.ServeHTTP(third, req)
	if third.Code != http.StatusOK {
		t.Fatalf("first request for different user: expected 200, got %d", third.Code)
	}
	if third.Header().Get("Retry-After") != "" {
		t.Fatal("did not expect Retry-After on allowed request")
	}
}

func TestAuthenticatedUserRateLimitRequiresUserContext(t *testing.T) {
	handler := AuthenticatedUserRateLimit(60, 1)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/files", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without authenticated user, got %d", rr.Code)
	}
}

func TestUserOrIPRateLimitFallsBackToIPForAnonymous(t *testing.T) {
	handler := UserOrIPRateLimit(60, 1, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/files", nil)
	req.RemoteAddr = "10.0.0.50:1234"
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first anonymous request: expected 200, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/files", nil)
	req.RemoteAddr = "10.0.0.50:1234"
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second anonymous request: expected 429, got %d", second.Code)
	}
}

func TestUserOrIPRateLimitPrefersUserIDOverIP(t *testing.T) {
	handler := UserOrIPRateLimit(60, 1, false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	userA := &domain.User{ID: uuid.New(), Role: domain.RoleUser}
	userB := &domain.User{ID: uuid.New(), Role: domain.RoleUser}

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/conversions", nil)
	req.RemoteAddr = "10.0.0.60:1234"
	req = req.WithContext(context.WithValue(req.Context(), userContextKey{}, userA))
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first request for user A: expected 200, got %d", first.Code)
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/conversions", nil)
	req.RemoteAddr = "10.0.0.60:1234"
	req = req.WithContext(context.WithValue(req.Context(), userContextKey{}, userB))
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusOK {
		t.Fatalf("first request for user B on same IP: expected 200, got %d", second.Code)
	}

	third := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/conversions", nil)
	req.RemoteAddr = "10.0.0.60:1234"
	req = req.WithContext(context.WithValue(req.Context(), userContextKey{}, userA))
	handler.ServeHTTP(third, req)
	if third.Code != http.StatusTooManyRequests {
		t.Fatalf("second request for user A: expected 429, got %d", third.Code)
	}
}
