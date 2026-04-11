package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/go-chi/chi/v5"
)

// MetricsMiddleware records HTTP request duration and status via Prometheus.
func MetricsMiddleware(m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(ww, r)

			// Use chi route pattern for lower-cardinality path label.
			path := chi.RouteContext(r.Context()).RoutePattern()
			if path == "" {
				path = "unmatched"
			}
			m.HTTPRequestDuration.WithLabelValues(
				r.Method,
				path,
				strconv.Itoa(ww.status),
			).Observe(time.Since(start).Seconds())
		})
	}
}
