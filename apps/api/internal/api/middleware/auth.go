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

// UserFromContext retrieves the authenticated user from request context.
func UserFromContext(ctx context.Context) *domain.User {
	u, _ := ctx.Value(userContextKey{}).(*domain.User)
	return u
}

// Auth returns middleware that validates a Bearer JWT and injects the user into context.
// If the token is missing or invalid, the request is rejected with 401.
func Auth(authSvc *auth.Service, users repository.UserRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid authorization header"})
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := authSvc.ValidateToken(tokenStr)
			if err != nil {
				respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
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
