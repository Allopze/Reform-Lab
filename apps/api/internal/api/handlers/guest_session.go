package handlers

import (
	"net/http"

	"github.com/google/uuid"
)

const guestSessionCookieName = "reform_guest_session"
const guestSessionCookieMaxAgeSeconds = 72 * 60 * 60

func ensureGuestSession(w http.ResponseWriter, r *http.Request, trustProxyHeaders bool) *uuid.UUID {
	if guestID := currentGuestSessionID(r); guestID != nil {
		return guestID
	}

	guestID := uuid.New()
	http.SetCookie(w, &http.Cookie{
		Name:     guestSessionCookieName,
		Value:    guestID.String(),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestUsesHTTPS(r, trustProxyHeaders),
		MaxAge:   guestSessionCookieMaxAgeSeconds,
	})
	return &guestID
}
