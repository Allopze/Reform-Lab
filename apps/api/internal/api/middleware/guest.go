package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type guestSessionContextKey struct{}

const guestSessionCookieName = "reform_guest_session"

// GuestSessionIDFromContext returns the parsed guest session ID, if present.
func GuestSessionIDFromContext(ctx context.Context) *uuid.UUID {
	guestID, _ := ctx.Value(guestSessionContextKey{}).(*uuid.UUID)
	return guestID
}

// OptionalGuestSession parses the anonymous guest session cookie and adds it to context.
func OptionalGuestSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie(guestSessionCookieName); err == nil {
			if guestID, parseErr := uuid.Parse(strings.TrimSpace(cookie.Value)); parseErr == nil {
				ctx := context.WithValue(r.Context(), guestSessionContextKey{}, &guestID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
