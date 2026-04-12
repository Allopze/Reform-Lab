package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/auth"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

type userContextKey struct{}

const sessionCookieName = "reform_session"

// UserFromContext retrieves the authenticated user from request context.
func UserFromContext(ctx context.Context) *domain.User {
	u, _ := ctx.Value(userContextKey{}).(*domain.User)
	return u
}

// Auth returns middleware that validates the session cookie and injects the user into context.
// If the session is missing or invalid, the request is rejected with 401.
func Auth(authSvc *auth.Service, users repository.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractToken(r)
			if tokenStr == "" {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing session cookie"})
				return
			}
			claims, err := authSvc.ValidateToken(tokenStr)
			if err != nil {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid session"})
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token subject"})
				return
			}

			u, err := users.GetByID(r.Context(), userID)
			if err != nil {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey{}, u)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth returns middleware that injects the user into context if a valid
// session cookie is present, but allows the request to proceed without authentication.
// Anonymous requests continue with a nil user in context.
func OptionalAuth(authSvc *auth.Service, users repository.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tokenStr := extractToken(r); tokenStr != "" {
				if claims, err := authSvc.ValidateToken(tokenStr); err == nil {
					if userID, err := uuid.Parse(claims.Subject); err == nil {
						if u, err := users.GetByID(r.Context(), userID); err == nil {
							ctx := context.WithValue(r.Context(), userContextKey{}, u)
							next.ServeHTTP(w, r.WithContext(ctx))
							return
						}
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func extractToken(r *http.Request) string {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		return strings.TrimSpace(cookie.Value)
	}
	return ""
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := UserFromContext(r.Context())
		if u == nil || !u.IsAdmin() {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// BearerTokenAuth protects a handler with a static bearer token.
// Returns 401 if the Authorization header doesn't match "Bearer <token>".
func BearerTokenAuth(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
				w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
