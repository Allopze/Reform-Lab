package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// User represents a registered account.
type User struct {
	ID              uuid.UUID  `json:"id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	Team            string     `json:"team,omitempty"`
	Role            UserRole   `json:"role"`
	IsSuspended     bool       `json:"isSuspended"`
	SuspendedReason *string    `json:"suspendedReason,omitempty"`
	EmailVerifiedAt *time.Time `json:"emailVerifiedAt,omitempty"`
	SessionVersion  int        `json:"sessionVersion"`
	PasswordHash    string     `json:"-"`
	CreatedAt       time.Time  `json:"createdAt"`
}

func (u User) IsAdmin() bool {
	return u.Role == RoleAdmin
}
