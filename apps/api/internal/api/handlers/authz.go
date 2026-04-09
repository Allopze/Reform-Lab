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

// userIDPtr returns a pointer to the user's ID, or nil if the user is nil (anonymous).
func userIDPtr(u *domain.User) *uuid.UUID {
	if u == nil {
		return nil
	}
	return &u.ID
}

func canAccessOwner(actor *domain.User, ownerID *uuid.UUID) bool {
	if ownerID == nil {
		return true
	}
	if actor == nil {
		return false
	}
	if actor.IsAdmin() {
		return true
	}
	return actor.ID == *ownerID
}
