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
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"engines": engines,
	})
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
