package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
)

const csrfCookieName = "reform_csrf"
const csrfHeaderName = "X-CSRF-Token"

// CSRF blocks cross-site mutating requests that rely on cookie authentication.
//
// Rationale: the API uses a session cookie (SameSite=Lax) and accepts credentials.
// This middleware validates browser origins and requires a double-submit CSRF
// token for browser mutating requests that include a session cookie.
//
// Compatibility: if no Origin/Referer/Sec-Fetch-Site headers are present, the
// request is allowed to avoid breaking non-browser clients.
func CSRF(allowedOrigin string, trustProxyHeaders bool) func(http.Handler) http.Handler {
	allowed := parseAllowedOrigins(allowedOrigin)
	if len(allowed) == 0 {
		allowed = []string{"http://localhost:3000"}
	}

	allowedSet := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		allowedSet[trimmed] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isMutatingMethod(r.Method) {
				ensureCSRFCookie(w, r, trustProxyHeaders)
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || strings.TrimSpace(cookie.Value) == "" {
				next.ServeHTTP(w, r)
				return
			}

			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" {
				if _, ok := allowedSet[origin]; !ok {
					respondJSON(w, http.StatusForbidden, map[string]string{"error": "csrf check failed"})
					return
				}
				if !validDoubleSubmitToken(r) {
					respondJSON(w, http.StatusForbidden, map[string]string{"error": "csrf token invalid"})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			refererOrigin := originFromReferer(r)
			if refererOrigin != "" {
				if _, ok := allowedSet[refererOrigin]; !ok {
					respondJSON(w, http.StatusForbidden, map[string]string{"error": "csrf check failed"})
					return
				}
				if !validDoubleSubmitToken(r) {
					respondJSON(w, http.StatusForbidden, map[string]string{"error": "csrf token invalid"})
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Fall back to Fetch Metadata when available.
			// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Sec-Fetch-Site
			site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")))
			switch site {
			case "":
				// No browser metadata: allow for non-browser clients.
				next.ServeHTTP(w, r)
				return
			case "same-origin", "same-site", "none":
				if !validDoubleSubmitToken(r) {
					respondJSON(w, http.StatusForbidden, map[string]string{"error": "csrf token invalid"})
					return
				}
				next.ServeHTTP(w, r)
				return
			default:
				respondJSON(w, http.StatusForbidden, map[string]string{"error": "csrf check failed"})
				return
			}
		})
	}
}

func ensureCSRFCookie(w http.ResponseWriter, r *http.Request, trustProxyHeaders bool) {
	if cookie, err := r.Cookie(csrfCookieName); err == nil && strings.TrimSpace(cookie.Value) != "" {
		return
	}
	token, err := newCSRFToken()
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestUsesHTTPS(r, trustProxyHeaders),
		MaxAge:   72 * 60 * 60,
	})
}

func validDoubleSubmitToken(r *http.Request) bool {
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil {
		return false
	}
	cookieValue := strings.TrimSpace(cookie.Value)
	headerValue := strings.TrimSpace(r.Header.Get(csrfHeaderName))
	if cookieValue == "" || headerValue == "" || len(cookieValue) != len(headerValue) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookieValue), []byte(headerValue)) == 1
}

func newCSRFToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func originFromReferer(r *http.Request) string {
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if referer == "" {
		return ""
	}
	u, err := url.Parse(referer)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
