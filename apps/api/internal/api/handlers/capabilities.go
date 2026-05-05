package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// CapabilitiesHandler handles GET /api/files/{fileId}/capabilities.
type CapabilitiesHandler struct {
	Files repository.FileRepository
}

type capabilityResponse struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	PresentationOrder int    `json:"presentationOrder"`
	TargetFormat      string `json:"targetFormat"`
	OperationType     string `json:"operationType"`
	TimeoutSeconds    int    `json:"timeoutSeconds"`
}

type catalogCapabilityResponse struct {
	ID                string   `json:"id"`
	DisplayName       string   `json:"displayName"`
	PresentationOrder int      `json:"presentationOrder"`
	SourceFormats     []string `json:"sourceFormats"`
	TargetFormat      string   `json:"targetFormat"`
	OperationType     string   `json:"operationType"`
	Family            string   `json:"family"`
	MaxInputBytes     int64    `json:"maxInputBytes"`
	TimeoutSeconds    int      `json:"timeoutSeconds"`
	MaxRetries        int      `json:"maxRetries"`
	ExpectedQuality   string   `json:"expectedQuality,omitempty"`
	KnownLimitations  []string `json:"knownLimitations,omitempty"`
}

type catalogFamilyResponse struct {
	Family       string                      `json:"family"`
	Capabilities []catalogCapabilityResponse `json:"capabilities"`
}

type batchCapabilitiesRequest struct {
	FileIDs []string `json:"fileIds"`
}

// CatalogHandler handles GET /api/catalog.
type CatalogHandler struct{}

func (h *CatalogHandler) Handle(w http.ResponseWriter, r *http.Request) {
	grouped := catalogByFamily(capabilities.Catalog)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"families": grouped,
	})
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
			ID:                c.ID,
			DisplayName:       c.DisplayName,
			PresentationOrder: c.PresentationOrder,
			TargetFormat:      c.TargetFormat,
			OperationType:     string(c.OperationType),
			TimeoutSeconds:    c.ExecutionLimits.TimeoutSeconds,
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": result,
	})
}

func (h *CapabilitiesHandler) HandleBatch(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	guestSessionID := currentGuestSessionID(r)

	var req batchCapabilitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(req.FileIDs) == 0 {
		respondError(w, http.StatusBadRequest, "at least one file ID is required")
		return
	}

	files := make([]*domain.OriginalFile, 0, len(req.FileIDs))
	for _, rawID := range req.FileIDs {
		fileID, err := uuid.Parse(rawID)
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
		files = append(files, file)
	}

	common := intersectCapabilities(files)
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"capabilities": common,
	})
}

func intersectCapabilities(files []*domain.OriginalFile) []capabilityResponse {
	if len(files) == 0 {
		return nil
	}

	counts := make(map[string]int)
	responses := make(map[string]capabilityResponse)
	ordered := make([]string, 0)

	for index, entry := range files {
		caps := capabilities.Resolve(*entry)
		seen := make(map[string]struct{})
		for _, cap := range caps {
			if _, ok := seen[cap.ID]; ok {
				continue
			}
			seen[cap.ID] = struct{}{}
			counts[cap.ID]++
			responses[cap.ID] = capabilityResponse{
				ID:                cap.ID,
				DisplayName:       cap.DisplayName,
				PresentationOrder: cap.PresentationOrder,
				TargetFormat:      cap.TargetFormat,
				OperationType:     string(cap.OperationType),
				TimeoutSeconds:    cap.ExecutionLimits.TimeoutSeconds,
			}
			if index == 0 {
				ordered = append(ordered, cap.ID)
			}
		}
	}

	result := make([]capabilityResponse, 0)
	for _, id := range ordered {
		if counts[id] == len(files) {
			result = append(result, responses[id])
		}
	}
	return result
}

func catalogByFamily(catalog []domain.Capability) []catalogFamilyResponse {
	families := make([]catalogFamilyResponse, 0)
	indexByFamily := make(map[domain.FormatFamily]int)

	for _, cap := range catalog {
		index, ok := indexByFamily[cap.Family]
		if !ok {
			index = len(families)
			indexByFamily[cap.Family] = index
			families = append(families, catalogFamilyResponse{
				Family:       string(cap.Family),
				Capabilities: []catalogCapabilityResponse{},
			})
		}
		families[index].Capabilities = append(families[index].Capabilities, catalogCapabilityResponse{
			ID:                cap.ID,
			DisplayName:       cap.DisplayName,
			PresentationOrder: cap.PresentationOrder,
			SourceFormats:     cap.SourceFormats,
			TargetFormat:      cap.TargetFormat,
			OperationType:     string(cap.OperationType),
			Family:            string(cap.Family),
			MaxInputBytes:     cap.SizeLimits.MaxInputBytes,
			TimeoutSeconds:    cap.ExecutionLimits.TimeoutSeconds,
			MaxRetries:        cap.ExecutionLimits.MaxRetries,
			ExpectedQuality:   cap.ExpectedQuality,
			KnownLimitations:  cap.KnownLimitations,
		})
	}

	return families
}
