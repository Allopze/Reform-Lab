package handlers

import (
	"net/http"
	"sort"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
)

type DashboardHandler struct {
	Dashboard repository.DashboardRepository
	Audit     repository.AuditRepository
}

type adminCapabilityAvailability struct {
	ID            string `json:"id"`
	DisplayName   string `json:"displayName"`
	Engine        string `json:"engine"`
	Family        string `json:"family"`
	OperationType string `json:"operationType"`
	TargetFormat  string `json:"targetFormat"`
	Available     bool   `json:"available"`
	Reason        string `json:"reason"`
}

func (h *DashboardHandler) Me(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	overview, err := h.Dashboard.GetUserDashboard(r.Context(), u.ID, 20)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load dashboard")
		return
	}

	respondJSON(w, http.StatusOK, overview)
}

func (h *DashboardHandler) AdminOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.Dashboard.GetAdminDashboard(r.Context(), 20)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load admin overview")
		return
	}

	audits, err := h.Audit.ListRecent(r.Context(), 20, nil)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load audit events")
		return
	}
	overview.RecentAudit = audits

	engines := capabilities.EffectiveEngineAvailability()
	overview.TotalEngines = len(engines)
	overview.UnavailableEngines = make([]string, 0)
	for name, available := range engines {
		if available {
			overview.AvailableEngines++
			continue
		}
		overview.UnavailableEngines = append(overview.UnavailableEngines, name)
	}
	sort.Strings(overview.UnavailableEngines)
	overview.EngineUsage = buildEngineUsage(overview.CapabilityUsage)

	respondJSON(w, http.StatusOK, overview)
}

func (h *DashboardHandler) AdminEngines(w http.ResponseWriter, r *http.Request) {
	engines := capabilities.EffectiveEngineAvailability()
	capabilityRows := buildAdminCapabilityAvailability()
	availableCapabilities := 0
	for _, capability := range capabilityRows {
		if capability.Available {
			availableCapabilities++
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"engines":               engines,
		"capabilities":          capabilityRows,
		"availableCapabilities": availableCapabilities,
		"totalCapabilities":     len(capabilityRows),
	})
}

func buildAdminCapabilityAvailability() []adminCapabilityAvailability {
	runtimeEngines := capabilities.DefaultProber.AvailableEngines()
	rows := make([]adminCapabilityAvailability, 0, len(capabilities.Catalog))

	for _, capability := range capabilities.Catalog {
		capabilityEnabled := capabilities.DefaultFlags.IsCapabilityEnabled(capability.ID)
		engineEnabled := capabilities.DefaultFlags.IsEngineEnabled(capability.Engine)
		engineRuntimeAvailable := runtimeEngines[capability.Engine]

		available := capabilityEnabled && engineEnabled && engineRuntimeAvailable
		reason := "available"
		if !capabilityEnabled {
			reason = "capability_disabled"
		} else if !engineRuntimeAvailable {
			reason = "engine_unavailable"
		} else if !engineEnabled {
			reason = "engine_disabled"
		}

		rows = append(rows, adminCapabilityAvailability{
			ID:            capability.ID,
			DisplayName:   capability.DisplayName,
			Engine:        capability.Engine,
			Family:        string(capability.Family),
			OperationType: string(capability.OperationType),
			TargetFormat:  capability.TargetFormat,
			Available:     available,
			Reason:        reason,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		leftOrder := 0
		rightOrder := 0
		if capability := capabilities.ByID(rows[i].ID); capability != nil {
			leftOrder = capability.PresentationOrder
		}
		if capability := capabilities.ByID(rows[j].ID); capability != nil {
			rightOrder = capability.PresentationOrder
		}
		if leftOrder == rightOrder {
			return rows[i].ID < rows[j].ID
		}
		return leftOrder < rightOrder
	})

	return rows
}

func buildEngineUsage(capabilityUsage []repository.AdminUsageStat) []repository.AdminUsageStat {
	aggregated := make(map[string]int)
	for _, item := range capabilityUsage {
		engine := "unknown"
		if capability := capabilities.ByID(item.Key); capability != nil {
			engine = capability.Engine
		}
		aggregated[engine] += item.Count
	}

	usage := make([]repository.AdminUsageStat, 0, len(aggregated))
	for key, count := range aggregated {
		usage = append(usage, repository.AdminUsageStat{Key: key, Count: count})
	}
	sort.Slice(usage, func(i, j int) bool {
		if usage[i].Count == usage[j].Count {
			return usage[i].Key < usage[j].Key
		}
		return usage[i].Count > usage[j].Count
	})
	return usage
}
