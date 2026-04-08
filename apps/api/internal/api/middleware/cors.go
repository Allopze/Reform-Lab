package middleware

import (
	"net/http"

	"github.com/go-chi/cors"
)

// CORS returns middleware configured for the given allowed origin.
func CORS(allowedOrigin string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   []string{allowedOrigin},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "Content-Disposition"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}
