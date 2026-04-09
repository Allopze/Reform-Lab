package handlers

import (
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

func TestCanAccessOwnerAllowsAnonymousResourcesByID(t *testing.T) {
	user := &domain.User{ID: uuid.New(), Role: domain.RoleUser}
	admin := &domain.User{ID: uuid.New(), Role: domain.RoleAdmin}

	if !canAccessOwner(nil, nil) {
		t.Fatal("expected anonymous actor to access anonymous resource")
	}
	if !canAccessOwner(admin, nil) {
		t.Fatal("expected admin to access anonymous resource")
	}
	if !canAccessOwner(user, nil) {
		t.Fatal("expected regular user to access anonymous resource")
	}
}
