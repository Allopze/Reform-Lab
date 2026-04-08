package domain

import (
	"time"

	"github.com/google/uuid"
)

// ConversionRequest represents a user's intention to execute a permitted operation.
type ConversionRequest struct {
	ID           uuid.UUID `json:"id"`
	FileID       uuid.UUID `json:"fileId"`
	CapabilityID string    `json:"capabilityId"`
	CreatedAt    time.Time `json:"createdAt"`
}
