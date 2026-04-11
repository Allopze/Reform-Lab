package handlers

import (
	"net/http"

	"github.com/allopze/reform-lab/apps/api/internal/api/middleware"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

func currentUser(r *http.Request) *domain.User {
	return middleware.UserFromContext(r.Context())
}

func currentGuestSessionID(r *http.Request) *uuid.UUID {
	return middleware.GuestSessionIDFromContext(r.Context())
}

// userIDPtr returns a pointer to the user's ID, or nil if the user is nil (anonymous).
func userIDPtr(u *domain.User) *uuid.UUID {
	if u == nil {
		return nil
	}
	return &u.ID
}

func canAccessResource(actor *domain.User, actorGuestSessionID, ownerID, resourceGuestSessionID *uuid.UUID) bool {
	if actor != nil && actor.IsAdmin() {
		return true
	}

	if ownerID != nil {
		if actor == nil {
			return false
		}
		return actor.ID == *ownerID
	}

	if actorGuestSessionID == nil || resourceGuestSessionID == nil {
		return false
	}
	return *actorGuestSessionID == *resourceGuestSessionID
}
