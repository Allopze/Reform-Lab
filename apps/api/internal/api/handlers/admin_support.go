package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

type AdminSupportHandler struct {
	RuntimeControls repository.RuntimeControlRepository
	Jobs            repository.JobRepository
	Orchestrator    *orchestrator.Service
	Workers         repository.WorkerStatusRepository
	Audit           repository.AuditRepository
}

type updateJobIntakeRequest struct {
	Paused bool   `json:"paused"`
	Reason string `json:"reason"`
}

func (h *AdminSupportHandler) UpdateJobIntake(w http.ResponseWriter, r *http.Request) {
	if h.RuntimeControls == nil {
		respondError(w, http.StatusNotImplemented, "runtime controls not configured")
		return
	}

	var req updateJobIntakeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var reason *string
	trimmedReason := strings.TrimSpace(req.Reason)
	if req.Paused {
		if trimmedReason == "" {
			respondError(w, http.StatusBadRequest, "reason is required when pausing intake")
			return
		}
		reason = &trimmedReason
	}

	cfgAdmin := userIDPtr(currentUser(r))
	if err := h.RuntimeControls.SetJobIntakePaused(r.Context(), req.Paused, reason, cfgAdmin); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update job intake state")
		return
	}

	state, err := h.RuntimeControls.Get(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load runtime controls")
		return
	}

	if req.Paused {
		h.auditSupportAction(r, domain.AuditAdminQueuePaused, map[string]interface{}{"reason": trimmedReason})
	} else {
		h.auditSupportAction(r, domain.AuditAdminQueueResumed, map[string]interface{}{})
	}

	respondJSON(w, http.StatusOK, state)
}

type drainQueuedJobsRequest struct {
	Limit int `json:"limit"`
}

func (h *AdminSupportHandler) DrainQueuedJobs(w http.ResponseWriter, r *http.Request) {
	if h.Jobs == nil || h.Orchestrator == nil {
		respondError(w, http.StatusNotImplemented, "job support controls not configured")
		return
	}

	var req drainQueuedJobsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	page, err := h.Jobs.ListForAdmin(r.Context(), repository.AdminJobFilter{
		Status: string(domain.JobQueued),
		Limit:  limit,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list queued jobs")
		return
	}

	cancelled := 0
	skipped := 0
	cancelledIDs := make([]string, 0, len(page.Jobs))
	for _, row := range page.Jobs {
		if err := h.Orchestrator.CancelJob(r.Context(), row.JobID); err != nil {
			skipped++
			continue
		}
		cancelled++
		cancelledIDs = append(cancelledIDs, row.JobID.String())
	}

	h.auditSupportAction(r, domain.AuditAdminQueueDrained, map[string]interface{}{
		"limit":        limit,
		"cancelled":    cancelled,
		"skipped":      skipped,
		"cancelledIds": cancelledIDs,
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"attempted":    len(page.Jobs),
		"cancelled":    cancelled,
		"skipped":      skipped,
		"cancelledIds": cancelledIDs,
	})
}

type pruneStaleWorkersRequest struct {
	StaleMinutes int `json:"staleMinutes"`
}

func (h *AdminSupportHandler) PruneStaleWorkers(w http.ResponseWriter, r *http.Request) {
	if h.Workers == nil {
		respondError(w, http.StatusNotImplemented, "worker support controls not configured")
		return
	}

	var req pruneStaleWorkersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	staleMinutes := req.StaleMinutes
	if staleMinutes <= 0 {
		staleMinutes = 60
	}
	if staleMinutes < 5 || staleMinutes > 7*24*60 {
		respondError(w, http.StatusBadRequest, "staleMinutes must be between 5 and 10080")
		return
	}

	cutoff := time.Now().UTC().Add(-time.Duration(staleMinutes) * time.Minute)
	deleted, err := h.Workers.DeleteStaleBefore(r.Context(), cutoff)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to prune stale workers")
		return
	}

	h.auditSupportAction(r, domain.AuditAdminWorkersPruned, map[string]interface{}{
		"staleMinutes": staleMinutes,
		"deleted":      deleted,
		"cutoff":       cutoff.Format(time.RFC3339),
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"deleted":      deleted,
		"staleMinutes": staleMinutes,
		"cutoff":       cutoff.Format(time.RFC3339),
	})
}

func (h *AdminSupportHandler) auditSupportAction(r *http.Request, eventType domain.AuditEventType, details map[string]interface{}) {
	if h.Audit == nil {
		return
	}
	adminID := ""
	if caller := currentUser(r); caller != nil {
		adminID = caller.ID.String()
	}
	payload := map[string]interface{}{"adminId": adminID}
	for key, value := range details {
		payload[key] = value
	}
	_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: eventType,
		Details:   payload,
		CreatedAt: time.Now().UTC(),
	})
}
