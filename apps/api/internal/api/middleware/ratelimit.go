package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"golang.org/x/time/rate"
)

// ipLimiter holds a rate.Limiter and the last time it was seen.
type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit applies a per-IP token-bucket rate limiter.
// rps is requests per second per IP; burst is the maximum burst size per IP.
// Stale entries are cleaned up every 3 minutes.
func RateLimit(rps float64, burst int, trustProxyHeaders bool, m ...*observability.Metrics) func(http.Handler) http.Handler {
	var reject func()
	if len(m) > 0 && m[0] != nil {
		reject = func() { m[0].RateLimitHits.WithLabelValues("ip").Inc() }
	}
	return keyedRateLimit(rps, burst, func(r *http.Request) (string, bool) {
		return realIP(r, trustProxyHeaders), true
	}, nil, "rate limit exceeded", reject)
}

// AuthenticatedUserRateLimit applies a token-bucket quota keyed by authenticated user ID.
// perMinute is the sustained quota per minute; burst controls how many immediate requests are allowed.
func AuthenticatedUserRateLimit(perMinute, burst int) func(http.Handler) http.Handler {
	if perMinute <= 0 || burst <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return keyedRateLimit(float64(perMinute)/60.0, burst, func(r *http.Request) (string, bool) {
		u := UserFromContext(r.Context())
		if u == nil {
			return "", false
		}
		return u.ID.String(), true
	}, func(w http.ResponseWriter) {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}, "user quota exceeded")
}

// UserOrIPRateLimit applies a quota keyed by authenticated user ID when present,
// and falls back to client IP for anonymous requests.
func UserOrIPRateLimit(perMinute, burst int, trustProxyHeaders bool, m ...*observability.Metrics) func(http.Handler) http.Handler {
	if perMinute <= 0 || burst <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	var reject func()
	if len(m) > 0 && m[0] != nil {
		reject = func() { m[0].RateLimitHits.WithLabelValues("user_or_ip").Inc() }
	}
	return keyedRateLimit(float64(perMinute)/60.0, burst, func(r *http.Request) (string, bool) {
		u := UserFromContext(r.Context())
		if u != nil {
			return "user:" + u.ID.String(), true
		}
		return "ip:" + realIP(r, trustProxyHeaders), true
	}, nil, "user quota exceeded", reject)
}

func keyedRateLimit(
	rps float64,
	burst int,
	keyFn func(*http.Request) (string, bool),
	onMissing func(http.ResponseWriter),
	errorMessage string,
	onReject ...func(), // optional callback when a request is rejected
) func(http.Handler) http.Handler {
	var mu sync.Mutex
	limiters := make(map[string]*rateLimiterEntry)

	// Background cleanup of stale entries.
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			mu.Lock()
			for key, entry := range limiters {
				if time.Since(entry.lastSeen) > 5*time.Minute {
					delete(limiters, key)
				}
			}
			mu.Unlock()
		}
	}()

	getLimiter := func(key string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()

		entry, exists := limiters[key]
		if !exists {
			entry = &rateLimiterEntry{
				limiter:  rate.NewLimiter(rate.Limit(rps), burst),
				lastSeen: time.Now(),
			}
			limiters[key] = entry
		}
		entry.lastSeen = time.Now()
		return entry.limiter
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key, ok := keyFn(r)
			if !ok {
				if onMissing != nil {
					onMissing(w)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			limiter := getLimiter(key)
			if !limiter.Allow() {
				if len(onReject) > 0 && onReject[0] != nil {
					onReject[0]()
				}
				w.Header().Set("Retry-After", "1")
				respondJSON(w, http.StatusTooManyRequests, map[string]string{
					"error": errorMessage,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// realIP extracts the client IP from the request, preferring
// X-Forwarded-For and X-Real-IP headers when behind a reverse proxy.
func realIP(r *http.Request, trustProxyHeaders bool) string {
	if trustProxyHeaders {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			for i := 0; i < len(xff); i++ {
				if xff[i] == ',' {
					return trimSpaces(xff[:i])
				}
			}
			return trimSpaces(xff)
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return trimSpaces(xri)
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func trimSpaces(value string) string {
	start := 0
	for start < len(value) && (value[start] == ' ' || value[start] == '\t') {
		start++
	}
	end := len(value)
	for end > start && (value[end-1] == ' ' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}

// MaxBodySize limits the request body to n bytes.
// Requests exceeding the limit will receive 413 Payload Too Large.
func MaxBodySize(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// SecurityHeaders adds standard security headers to all responses.
func SecurityHeaders(next http.Handler) http.Handler {
	return SecurityHeadersWithTrustProxy(false)(next)
}

func SecurityHeadersWithTrustProxy(trustProxyHeaders bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "0")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
			if requestUsesHTTPS(r, trustProxyHeaders) {
				w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requestUsesHTTPS(r *http.Request, trustProxyHeaders bool) bool {
	if r.TLS != nil {
		return true
	}
	return trustProxyHeaders && strings.EqualFold(trimSpaces(r.Header.Get("X-Forwarded-Proto")), "https")
}
