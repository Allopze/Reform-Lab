package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/google/uuid"
)

type AdminJobsHandler struct {
	Jobs         repository.JobRepository
	Orchestrator *orchestrator.Service
	Files        repository.FileRepository
	Store        storage.Store
	Audit        repository.AuditRepository
}

type adminBatchJobActionRequest struct {
	JobIDs []string                   `json:"jobIds"`
	Filter *repository.AdminJobFilter `json:"filter"`
}

const maxAdminBatchJobAction = 500

func (h *AdminJobsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	stalledOnly, _ := strconv.ParseBool(q.Get("stalled"))

	filter := repository.AdminJobFilter{
		Status:       q.Get("status"),
		CapabilityID: q.Get("capability"),
		Search:       q.Get("q"),
		StalledOnly:  stalledOnly,
		Limit:        limit,
		Offset:       offset,
	}

	page, err := h.Jobs.ListForAdmin(r.Context(), filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	respondJSON(w, http.StatusOK, page)
}

func (h *AdminJobsHandler) CancelBatch(w http.ResponseWriter, r *http.Request) {
	jobIDs, jobs, mode, err := h.resolveBatchJobs(r)
	if err != nil {
		respondError(w, err.status, err.message)
		return
	}
	for _, job := range jobs {
		if cancelErr := h.Orchestrator.CancelJob(r.Context(), job.ID); cancelErr != nil {
			respondError(w, http.StatusConflict, "cannot cancel one or more jobs in current state")
			return
		}
	}
	h.auditBatch(r, domain.AuditAdminJobsCancelled, mode, jobIDs)
	respondJSON(w, http.StatusOK, map[string]interface{}{"cancelledJobIds": jobIDs})
}

func (h *AdminJobsHandler) RetryBatch(w http.ResponseWriter, r *http.Request) {
	jobIDs, jobs, mode, err := h.resolveBatchJobs(r)
	if err != nil {
		respondError(w, err.status, err.message)
		return
	}
	created := make([]*domain.Job, 0, len(jobs))
	for _, job := range jobs {
		if job.Status != domain.JobFailed {
			respondError(w, http.StatusConflict, "only failed jobs can be retried")
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
		var retried *domain.Job
		var retryErr error
		if file.UserID == nil && file.GuestSessionID != nil {
			retried, retryErr = h.Orchestrator.RetryFailedJobForGuest(r.Context(), *file.GuestSessionID, job, *capability, inputPath, file.Size)
		} else {
			retried, retryErr = h.Orchestrator.RetryFailedJob(r.Context(), job, *capability, inputPath, file.Size)
		}
		if retryErr != nil {
			if errors.Is(retryErr, domain.ErrJobIntakePaused) {
				respondError(w, http.StatusServiceUnavailable, "job intake is temporarily paused by admin")
				return
			}
			if errors.Is(retryErr, domain.ErrTooManyActiveJobs) {
				respondError(w, http.StatusTooManyRequests, "too many active jobs for one or more users")
				return
			}
			respondError(w, http.StatusConflict, "failed to retry one or more jobs")
			return
		}
		created = append(created, retried)
	}
	h.auditBatch(r, domain.AuditAdminJobsRetried, mode, jobIDs)
	respondJSON(w, http.StatusCreated, map[string]interface{}{"jobs": created})
}

type adminBatchResolveError struct {
	status  int
	message string
}

func (h *AdminJobsHandler) resolveBatchJobs(r *http.Request) ([]string, []*domain.Job, string, *adminBatchResolveError) {
	var req adminBatchJobActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, nil, "", &adminBatchResolveError{status: http.StatusBadRequest, message: "invalid request body"}
	}
	ids := make([]uuid.UUID, 0)
	mode := "selection"
	if len(req.JobIDs) > 0 {
		for _, rawID := range req.JobIDs {
			jobID, err := uuid.Parse(rawID)
			if err != nil {
				return nil, nil, "", &adminBatchResolveError{status: http.StatusBadRequest, message: "invalid job ID"}
			}
			ids = append(ids, jobID)
		}
	} else if req.Filter != nil {
		mode = "filter"
		filter := *req.Filter
		filter.Search = strings.TrimSpace(filter.Search)
		matched, err := h.Jobs.ListIDsForAdmin(r.Context(), filter)
		if err != nil {
			return nil, nil, "", &adminBatchResolveError{status: http.StatusInternalServerError, message: "failed to resolve jobs"}
		}
		ids = matched
	} else {
		return nil, nil, "", &adminBatchResolveError{status: http.StatusBadRequest, message: "jobIds or filter is required"}
	}
	if len(ids) == 0 {
		return nil, nil, "", &adminBatchResolveError{status: http.StatusBadRequest, message: "no jobs matched the requested action"}
	}
	if len(ids) > maxAdminBatchJobAction {
		return nil, nil, "", &adminBatchResolveError{status: http.StatusRequestEntityTooLarge, message: "too many jobs matched the requested action"}
	}
	jobIDs := make([]string, 0, len(ids))
	jobs := make([]*domain.Job, 0, len(ids))
	for _, id := range ids {
		job, err := h.Orchestrator.GetJob(r.Context(), id)
		if err != nil {
			return nil, nil, "", &adminBatchResolveError{status: http.StatusNotFound, message: "job not found"}
		}
		jobIDs = append(jobIDs, id.String())
		jobs = append(jobs, job)
	}
	return jobIDs, jobs, mode, nil
}

func (h *AdminJobsHandler) auditBatch(r *http.Request, eventType domain.AuditEventType, mode string, jobIDs []string) {
	adminID := ""
	if caller := currentUser(r); caller != nil {
		adminID = caller.ID.String()
	}
	_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: eventType,
		Details: map[string]interface{}{
			"adminId": adminID,
			"mode":    mode,
			"jobIds":  jobIDs,
			"count":   len(jobIDs),
		},
		CreatedAt: time.Now().UTC(),
	})
}
