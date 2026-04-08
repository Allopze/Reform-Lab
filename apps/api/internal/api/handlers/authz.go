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

func canAccessOwner(actor *domain.User, ownerID *uuid.UUID) bool {
	if actor == nil {
		return false
	}
	if actor.IsAdmin() {
		return true
	}
	if ownerID == nil {
		return false
	}
	return actor.ID == *ownerID
}
