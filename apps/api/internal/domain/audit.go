package domain

import (
	"time"

	"github.com/google/uuid"
)

// AuditEventType classifies audit entries.
type AuditEventType string

const (
	AuditUpload          AuditEventType = "upload"
	AuditDetection       AuditEventType = "detection"
	AuditJobCreated      AuditEventType = "job_created"
	AuditJobStarted      AuditEventType = "job_started"
	AuditJobCompleted    AuditEventType = "job_completed"
	AuditJobFailed       AuditEventType = "job_failed"
	AuditJobCancelled    AuditEventType = "job_cancelled"
	AuditJobRetried      AuditEventType = "job_retried"
	AuditArtifactCreated AuditEventType = "artifact_created"
)

// AuditEvent is a structured record for operational and security traceability.
type AuditEvent struct {
	ID        uuid.UUID              `json:"id"`
	EventType AuditEventType         `json:"eventType"`
	FileID    *uuid.UUID             `json:"fileId,omitempty"`
	JobID     *uuid.UUID             `json:"jobId,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
}
