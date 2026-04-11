package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/orchestrator"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// ConversionHandler handles POST /api/conversions.
type ConversionHandler struct {
	Files        repository.FileRepository
	Store        storage.Store
	Orchestrator *orchestrator.Service
	Logger       zerolog.Logger
}

type conversionRequest struct {
	FileID       string `json:"fileId"`
	CapabilityID string `json:"capabilityId"`
}

func (h *ConversionHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r) // may be nil for anonymous users
	guestSessionID := currentGuestSessionID(r)

	var req conversionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	fileID, err := uuid.Parse(req.FileID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid file ID")
		return
	}

	file, err := h.Files.GetByID(r.Context(), fileID)
	if err != nil {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}
	if !canAccessResource(u, guestSessionID, file.UserID, file.GuestSessionID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}

	// Validate capability is eligible for this file (prevents tampering).
	cap, err := capabilities.IsEligible(*file, req.CapabilityID)
	if err != nil {
		switch err {
		case domain.ErrCapabilityNotFound:
			respondError(w, http.StatusBadRequest, "unknown capability")
		case domain.ErrCapabilityIneligible:
			respondError(w, http.StatusBadRequest, "capability not available for this file")
		case domain.ErrLimitExceeded:
			respondError(w, http.StatusRequestEntityTooLarge, "file exceeds capability limits")
		default:
			respondError(w, http.StatusBadRequest, "capability check failed")
		}
		return
	}

	inputPath := h.Store.OriginalPath(fileID.String())

	job, err := h.Orchestrator.CreateAndEnqueue(r.Context(), userIDPtr(u), fileID, *cap, inputPath)
	if err != nil {
		if errors.Is(err, domain.ErrTooManyActiveJobs) {
			respondError(w, http.StatusTooManyRequests, "too many active jobs for this user")
			return
		}
		h.Logger.Error().Err(err).Str("file_id", fileID.String()).Msg("failed to create job")
		respondError(w, http.StatusInternalServerError, "failed to create conversion job")
		return
	}

	respondJSON(w, http.StatusCreated, job)
}
