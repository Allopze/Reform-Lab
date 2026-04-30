package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/cors"
)

// CORS returns middleware configured for the given allowed origin.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	origins := parseAllowedOrigins(allowedOrigin)
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}

	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "X-Request-ID", "X-CSRF-Token"},
		ExposedHeaders:   []string{"X-Request-ID", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

func parseAllowedOrigins(value string) []string {
	parts := strings.Split(value, ",")
	origins := make([]string, 0, len(parts))

	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		origins = append(origins, origin)
	}

	return origins
}
