package handlers

import (
	"net/http"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// JobHandler handles job-related endpoints.
type JobHandler struct {
	Orchestrator *orchestrator.Service
	Files        repository.FileRepository
	Store        storage.Store
}

func (h *JobHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.Orchestrator.GetJob(r.Context(), jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}
	if !canAccessOwner(u, job.UserID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	respondJSON(w, http.StatusOK, job)
}

// Cancel handles POST /api/jobs/{jobId}/cancel.
func (h *JobHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.Orchestrator.GetJob(r.Context(), jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}
	if !canAccessOwner(u, job.UserID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	if err := h.Orchestrator.CancelJob(r.Context(), jobID); err != nil {
		respondError(w, http.StatusConflict, "cannot cancel job in current state")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// Retry handles POST /api/jobs/{jobId}/retry.
func (h *JobHandler) Retry(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	jobID, err := uuid.Parse(chi.URLParam(r, "jobId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid job ID")
		return
	}

	job, err := h.Orchestrator.GetJob(r.Context(), jobID)
	if err != nil {
		respondError(w, http.StatusNotFound, "job not found")
		return
	}
	if !canAccessOwner(u, job.UserID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if job.Status != domain.JobFailed {
		respondError(w, http.StatusConflict, "only failed jobs can be retried")
		return
	}

	file, err := h.Files.GetByID(r.Context(), job.FileID)
	if err != nil {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}
	if !canAccessOwner(u, file.UserID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	capability, err := capabilities.IsEligible(*file, job.CapabilityID)
	if err != nil {
		respondError(w, http.StatusConflict, "capability no longer eligible for retry")
		return
	}

	retryJob, err := h.Orchestrator.RetryFailedJob(r.Context(), job, *capability, h.Store.OriginalPath(job.FileID.String()))
	if err != nil {
		respondError(w, http.StatusConflict, "failed to retry job")
		return
	}

	respondJSON(w, http.StatusCreated, retryJob)
}
