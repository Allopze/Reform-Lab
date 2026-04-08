package domain

import (
	"time"

	"github.com/google/uuid"
)

// Artifact is a persisted result of a successful conversion.
type Artifact struct {
	ID          uuid.UUID  `json:"id"`
	UserID      *uuid.UUID `json:"userId,omitempty"`
	JobID       uuid.UUID  `json:"jobId"`
	FileID      uuid.UUID  `json:"fileId"`
	FileName    string     `json:"fileName"`
	MIMEType    string     `json:"mimeType"`
	Size        int64      `json:"size"`
	StoragePath string     `json:"-"`
	CreatedAt   time.Time  `json:"createdAt"`
	ExpiresAt   time.Time  `json:"expiresAt"`
}
