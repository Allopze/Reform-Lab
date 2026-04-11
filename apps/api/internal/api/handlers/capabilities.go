package handlers

import (
	"net/http"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CapabilitiesHandler handles GET /api/files/{fileId}/capabilities.
type CapabilitiesHandler struct {
	Files repository.FileRepository
}

type capabilityResponse struct {
	ID             string `json:"id"`
	DisplayName    string `json:"displayName"`
	TargetFormat   string `json:"targetFormat"`
	OperationType  string `json:"operationType"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
}

func (h *CapabilitiesHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r) // may be nil for anonymous users
	guestSessionID := currentGuestSessionID(r)

	fileID, err := uuid.Parse(chi.URLParam(r, "fileId"))
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

	caps := capabilities.Resolve(*file)

	result := make([]capabilityResponse, len(caps))
	for i, c := range caps {
		result[i] = capabilityResponse{
			ID:             c.ID,
			DisplayName:    c.DisplayName,
			TargetFormat:   c.TargetFormat,
			OperationType:  string(c.OperationType),
			TimeoutSeconds: c.ExecutionLimits.TimeoutSeconds,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": result,
	})
}
