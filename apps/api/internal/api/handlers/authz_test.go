package handlers

import (
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/google/uuid"
)

func TestCanAccessResource(t *testing.T) {
	user := &domain.User{ID: uuid.New(), Role: domain.RoleUser}
	admin := &domain.User{ID: uuid.New(), Role: domain.RoleAdmin}
	ownerID := user.ID
	guestID := uuid.New()
	otherGuestID := uuid.New()

	if !canAccessResource(admin, nil, nil, nil) {
		t.Fatal("expected admin to access any resource")
	}
	if !canAccessResource(user, nil, &ownerID, nil) {
		t.Fatal("expected owner to access owned resource")
	}
	if canAccessResource(nil, nil, nil, &guestID) {
		t.Fatal("expected anonymous actor without guest session to be rejected")
	}
	if !canAccessResource(nil, &guestID, nil, &guestID) {
		t.Fatal("expected matching guest session to access guest resource")
	}
	if canAccessResource(nil, &otherGuestID, nil, &guestID) {
		t.Fatal("expected different guest session to be rejected")
	}
}
