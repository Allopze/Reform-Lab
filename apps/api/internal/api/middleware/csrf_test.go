package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFBLocksCrossSiteOriginWhenSessionCookiePresent(t *testing.T) {
	h := CSRF("http://good.test", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/footer-message", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	req.Header.Set("Origin", "http://evil.test")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.Code)
	}
}

func TestCSRFAcceptsAllowedOriginWhenSessionCookiePresent(t *testing.T) {
	h := CSRF("http://good.test", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/footer-message", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	req.Header.Set("Origin", "http://good.test")
	req.Header.Set(csrfHeaderName, "csrf-token")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}
}

func TestCSRFSkipsWhenNoSessionCookiePresent(t *testing.T) {
	h := CSRF("http://good.test", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/conversions", nil)
	req.Header.Set("Origin", "http://evil.test")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}
}

func TestCSRFAcceptsRefererWhenOriginMissing(t *testing.T) {
	h := CSRF("http://good.test", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/jobs/123/cancel", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-token"})
	req.Header.Set("Referer", "http://good.test/app")
	req.Header.Set(csrfHeaderName, "csrf-token")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}
}

func TestCSRFBLocksCrossSiteFetchMetadataWhenOriginMissing(t *testing.T) {
	h := CSRF("http://good.test", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/footer-message", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.Code)
	}
}

func TestCSRFRejectsAllowedOriginWithoutDoubleSubmitToken(t *testing.T) {
	h := CSRF("http://good.test", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/admin/footer-message", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	req.Header.Set("Origin", "http://good.test")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.Code)
	}
}

func TestCSRFIssuesCookieOnSafeRequest(t *testing.T) {
	h := CSRF("http://good.test", false)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", res.Code)
	}
	if cookies := res.Result().Cookies(); len(cookies) == 0 || cookies[0].Name != csrfCookieName {
		t.Fatalf("expected csrf cookie, got %#v", cookies)
	}
}
