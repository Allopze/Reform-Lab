package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"

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
	Artifacts    repository.ArtifactRepository
	Files        repository.FileRepository
	Store        storage.Store
}

type jobResponse struct {
	*domain.Job
	ArtifactFileName *string `json:"artifactFileName,omitempty"`
	ArtifactMIMEType *string `json:"artifactMimeType,omitempty"`
	ArtifactSize     *int64  `json:"artifactSize,omitempty"`
}

type batchJobActionRequest struct {
	JobIDs []string `json:"jobIds"`
}

func (h *JobHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r) // may be nil for anonymous users
	guestSessionID := currentGuestSessionID(r)

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
	file, err := h.Files.GetByID(r.Context(), job.FileID)
	if err != nil {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}
	if !canAccessResource(u, guestSessionID, file.UserID, file.GuestSessionID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	respondJSON(w, http.StatusOK, h.buildJobResponse(r, job))
}

// Cancel handles POST /api/jobs/{jobId}/cancel.
func (h *JobHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r) // may be nil for anonymous users
	guestSessionID := currentGuestSessionID(r)

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
	file, err := h.Files.GetByID(r.Context(), job.FileID)
	if err != nil {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}
	if !canAccessResource(u, guestSessionID, file.UserID, file.GuestSessionID) {
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
	u := currentUser(r) // may be nil for anonymous users
	guestSessionID := currentGuestSessionID(r)

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
	if job.Status != domain.JobFailed {
		respondError(w, http.StatusConflict, "only failed jobs can be retried")
		return
	}

	file, err := h.Files.GetByID(r.Context(), job.FileID)
	if err != nil {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}
	if !canAccessResource(u, guestSessionID, file.UserID, file.GuestSessionID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	capability, err := capabilities.IsEligible(*file, job.CapabilityID)
	if err != nil {
		respondError(w, http.StatusConflict, "capability no longer eligible for retry")
		return
	}

	inputPath, ok := originalPathIfAvailable(h.Store, job.FileID)
	if !ok {
		respondError(w, http.StatusGone, "original file expired or unavailable")
		return
	}

	var retryJob *domain.Job
	if file.UserID == nil && file.GuestSessionID != nil {
		retryJob, err = h.Orchestrator.RetryFailedJobForGuest(r.Context(), *file.GuestSessionID, job, *capability, inputPath, file.Size)
	} else {
		retryJob, err = h.Orchestrator.RetryFailedJob(r.Context(), job, *capability, inputPath, file.Size)
	}
	if err != nil {
		if errors.Is(err, domain.ErrJobIntakePaused) {
			respondError(w, http.StatusServiceUnavailable, "job intake is temporarily paused by admin")
			return
		}
		if errors.Is(err, domain.ErrTooManyActiveJobs) {
			respondError(w, http.StatusTooManyRequests, "too many active jobs for this user")
			return
		}
		respondError(w, http.StatusConflict, "failed to retry job")
		return
	}

	respondJSON(w, http.StatusCreated, retryJob)
}

func (h *JobHandler) CancelBatch(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	guestSessionID := currentGuestSessionID(r)

	jobIDs, jobs, _, _, err := h.batchTargetJobs(r, u, guestSessionID)
	if err != nil {
		respondError(w, err.status, err.message)
		return
	}

	for _, job := range jobs {
		if err := h.Orchestrator.CancelJob(r.Context(), job.ID); err != nil {
			respondError(w, http.StatusConflict, "cannot cancel one or more jobs in current state")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{"cancelledJobIds": jobIDs})
}

func (h *JobHandler) RetryBatch(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	guestSessionID := currentGuestSessionID(r)

	_, jobs, fileOwner, batchGuestSessionID, err := h.batchTargetJobs(r, u, guestSessionID)
	if err != nil {
		respondError(w, err.status, err.message)
		return
	}

	requests := make([]orchestrator.BatchRequest, 0, len(jobs))
	for _, job := range jobs {
		if job.Status != domain.JobFailed {
			respondError(w, http.StatusConflict, "only failed jobs can be retried in batch")
			return
		}

		file, getErr := h.Files.GetByID(r.Context(), job.FileID)
		if getErr != nil {
			respondError(w, http.StatusNotFound, "file not found")
			return
		}

		capability, capErr := capabilities.IsEligible(*file, job.CapabilityID)
		if capErr != nil {
			respondError(w, http.StatusConflict, "capability no longer eligible for one or more retries")
			return
		}

		inputPath, ok := originalPathIfAvailable(h.Store, job.FileID)
		if !ok {
			respondError(w, http.StatusGone, "original file expired or unavailable")
			return
		}

		requests = append(requests, orchestrator.BatchRequest{
			FileID:     job.FileID,
			Capability: *capability,
			InputPath:  inputPath,
		})
	}

	var retriedJobs []domain.Job
	var retryErr error
	if fileOwner == nil && batchGuestSessionID != nil {
		retriedJobs, retryErr = h.Orchestrator.CreateAndEnqueueBatchForGuest(r.Context(), *batchGuestSessionID, requests)
	} else {
		retriedJobs, retryErr = h.Orchestrator.CreateAndEnqueueBatch(r.Context(), fileOwner, requests)
	}
	if retryErr != nil {
		if errors.Is(retryErr, domain.ErrJobIntakePaused) {
			respondError(w, http.StatusServiceUnavailable, "job intake is temporarily paused by admin")
			return
		}
		if errors.Is(retryErr, domain.ErrTooManyActiveJobs) {
			respondError(w, http.StatusTooManyRequests, "too many active jobs for this user")
			return
		}
		respondError(w, http.StatusConflict, "failed to retry jobs")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{"jobs": retriedJobs})
}

type batchJobTargetsError struct {
	status  int
	message string
}

func (h *JobHandler) batchTargetJobs(r *http.Request, u *domain.User, guestSessionID *uuid.UUID) ([]string, []*domain.Job, *uuid.UUID, *uuid.UUID, *batchJobTargetsError) {
	var req batchJobActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, nil, nil, nil, &batchJobTargetsError{status: http.StatusBadRequest, message: "invalid request body"}
	}
	if len(req.JobIDs) == 0 {
		return nil, nil, nil, nil, &batchJobTargetsError{status: http.StatusBadRequest, message: "at least one job ID is required"}
	}

	jobIDs := make([]string, 0, len(req.JobIDs))
	jobs := make([]*domain.Job, 0, len(req.JobIDs))
	var ownerID *uuid.UUID
	var ownerGuestSessionID *uuid.UUID
	for _, rawID := range req.JobIDs {
		jobID, err := uuid.Parse(rawID)
		if err != nil {
			return nil, nil, nil, nil, &batchJobTargetsError{status: http.StatusBadRequest, message: "invalid job ID"}
		}

		job, err := h.Orchestrator.GetJob(r.Context(), jobID)
		if err != nil {
			return nil, nil, nil, nil, &batchJobTargetsError{status: http.StatusNotFound, message: "job not found"}
		}
		file, err := h.Files.GetByID(r.Context(), job.FileID)
		if err != nil {
			return nil, nil, nil, nil, &batchJobTargetsError{status: http.StatusNotFound, message: "file not found"}
		}
		if !canAccessResource(u, guestSessionID, file.UserID, file.GuestSessionID) {
			return nil, nil, nil, nil, &batchJobTargetsError{status: http.StatusForbidden, message: "forbidden"}
		}
		if len(jobs) == 0 {
			ownerID = job.UserID
			ownerGuestSessionID = file.GuestSessionID
		} else if !sameOptionalUUID(ownerID, job.UserID) || !sameOptionalUUID(ownerGuestSessionID, file.GuestSessionID) {
			return nil, nil, nil, nil, &batchJobTargetsError{status: http.StatusBadRequest, message: "all jobs in a batch action must share the same owner"}
		}

		jobIDs = append(jobIDs, jobID.String())
		jobs = append(jobs, job)
	}

	return jobIDs, jobs, ownerID, ownerGuestSessionID, nil
}

func sameOptionalUUID(left, right *uuid.UUID) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func originalPathIfAvailable(store storage.Store, fileID uuid.UUID) (string, bool) {
	path := store.OriginalPath(fileID.String())
	if _, err := os.Stat(path); err != nil {
		return path, false
	}
	return path, true
}

func (h *JobHandler) buildJobResponse(r *http.Request, job *domain.Job) *jobResponse {
	response := &jobResponse{Job: job}
	if job == nil || job.ArtifactID == nil || h.Artifacts == nil {
		return response
	}

	artifact, err := h.Artifacts.GetByID(r.Context(), *job.ArtifactID)
	if err != nil {
		return response
	}

	response.ArtifactFileName = &artifact.FileName
	response.ArtifactMIMEType = &artifact.MIMEType
	response.ArtifactSize = &artifact.Size
	return response
}
