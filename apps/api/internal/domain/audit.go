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

	// Admin mutation events
	AuditAdminFooterUpdated   AuditEventType = "admin_footer_updated"
	AuditAdminUploadPolicy    AuditEventType = "admin_upload_policy_updated"
	AuditAdminSMTPUpdated     AuditEventType = "admin_smtp_updated"
	AuditAdminSMTPTest        AuditEventType = "admin_smtp_test"
	AuditAdminTemplateCreated AuditEventType = "admin_template_created"
	AuditAdminTemplateUpdated AuditEventType = "admin_template_updated"
	AuditAdminTemplateDeleted AuditEventType = "admin_template_deleted"
	AuditAdminWebhookCreated  AuditEventType = "admin_webhook_created"
	AuditAdminWebhookUpdated  AuditEventType = "admin_webhook_updated"
	AuditAdminWebhookDeleted  AuditEventType = "admin_webhook_deleted"
	AuditAdminRoleChanged     AuditEventType = "admin_role_changed"
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
