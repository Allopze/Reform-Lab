package domain

import (
	"time"

	"github.com/google/uuid"
)

// FormatFamily groups detected formats into broad categories.
type FormatFamily string

const (
	FamilyPDF      FormatFamily = "pdf"
	FamilyImage    FormatFamily = "image"
	FamilyDocument FormatFamily = "document"
	FamilyAudio    FormatFamily = "audio"
	FamilyVideo    FormatFamily = "video"
)

// DetectedFormat represents the real format determined by content inspection,
// never by file extension alone.
type DetectedFormat struct {
	MIMEType   string       `json:"mimeType"`
	Family     FormatFamily `json:"family"`
	Extension  string       `json:"extension"`
	Confidence float64      `json:"confidence,omitempty"`
}

// FileMetadata holds format-specific metadata extracted during ingestion.
type FileMetadata struct {
	Pages       *int     `json:"pages,omitempty"`
	Width       *int     `json:"width,omitempty"`
	Height      *int     `json:"height,omitempty"`
	DurationSec *float64 `json:"durationSec,omitempty"`
	Encoding    *string  `json:"encoding,omitempty"`
	IsProtected bool     `json:"isProtected,omitempty"`
}

// OriginalFile is the immutable record of an uploaded file.
type OriginalFile struct {
	ID             uuid.UUID      `json:"id"`
	UserID         *uuid.UUID     `json:"userId,omitempty"`
	GuestSessionID *uuid.UUID     `json:"-"`
	InternalName   string         `json:"-"`
	OriginalName   string         `json:"originalName"`
	Size           int64          `json:"size"`
	DetectedFormat DetectedFormat `json:"detectedFormat"`
	Metadata       FileMetadata   `json:"metadata"`
	UploadedAt     time.Time      `json:"uploadedAt"`
}
