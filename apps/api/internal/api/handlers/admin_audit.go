package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/repository"
)

const (
	defaultAuditPageSize = 50
	maxAuditPageSize     = 200
	defaultAuditCSVLimit = 2000
	maxAuditCSVLimit     = 10000
)

type AdminAuditHandler struct {
	Audit repository.AuditRepository
}

type adminAuditPageResponse struct {
	Events []adminAuditEventResponse `json:"events"`
	Total  int                       `json:"total"`
}

type adminAuditEventResponse struct {
	ID        string                 `json:"id"`
	EventType string                 `json:"eventType"`
	FileID    *string                `json:"fileId,omitempty"`
	JobID     *string                `json:"jobId,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty"`
	CreatedAt string                 `json:"createdAt"`
}

func (h *AdminAuditHandler) List(w http.ResponseWriter, r *http.Request) {
	if h.Audit == nil {
		respondError(w, http.StatusInternalServerError, "audit repository is not configured")
		return
	}

	group, ok := parseAuditGroup(r.URL.Query().Get("group"))
	if !ok {
		respondError(w, http.StatusBadRequest, "group must be 'admin' when provided")
		return
	}

	limit, err := parseBoundedInt(r.URL.Query().Get("limit"), defaultAuditPageSize, maxAuditPageSize)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid limit")
		return
	}
	offset, err := parseBoundedInt(r.URL.Query().Get("offset"), 0, int(^uint(0)>>1))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid offset")
		return
	}

	page, err := h.Audit.ListForAdmin(r.Context(), repository.AdminAuditFilter{
		EventType: strings.TrimSpace(r.URL.Query().Get("eventType")),
		Prefix:    group,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list audit events")
		return
	}

	result := make([]adminAuditEventResponse, 0, len(page.Events))
	for _, event := range page.Events {
		row := adminAuditEventResponse{
			ID:        event.ID.String(),
			EventType: string(event.EventType),
			Details:   event.Details,
			CreatedAt: event.CreatedAt.Format(time.RFC3339),
		}
		if event.FileID != nil {
			fileID := event.FileID.String()
			row.FileID = &fileID
		}
		if event.JobID != nil {
			jobID := event.JobID.String()
			row.JobID = &jobID
		}
		result = append(result, row)
	}

	respondJSON(w, http.StatusOK, adminAuditPageResponse{Events: result, Total: page.Total})
}

func (h *AdminAuditHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	if h.Audit == nil {
		respondError(w, http.StatusInternalServerError, "audit repository is not configured")
		return
	}

	group, ok := parseAuditGroup(r.URL.Query().Get("group"))
	if !ok {
		respondError(w, http.StatusBadRequest, "group must be 'admin' when provided")
		return
	}

	limit, err := parseBoundedInt(r.URL.Query().Get("limit"), defaultAuditCSVLimit, maxAuditCSVLimit)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	page, err := h.Audit.ListForAdmin(r.Context(), repository.AdminAuditFilter{
		EventType: strings.TrimSpace(r.URL.Query().Get("eventType")),
		Prefix:    group,
		Limit:     limit,
		Offset:    0,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to export audit events")
		return
	}

	filename := fmt.Sprintf("admin-audit-%s.csv", time.Now().UTC().Format("20060102-150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"id", "eventType", "fileId", "jobId", "createdAt", "detailsJson"})
	for _, event := range page.Events {
		fileID := ""
		if event.FileID != nil {
			fileID = event.FileID.String()
		}
		jobID := ""
		if event.JobID != nil {
			jobID = event.JobID.String()
		}
		detailsJSON := "{}"
		if len(event.Details) > 0 {
			if data, err := json.Marshal(event.Details); err == nil {
				detailsJSON = string(data)
			}
		}

		_ = writer.Write([]string{
			event.ID.String(),
			string(event.EventType),
			fileID,
			jobID,
			event.CreatedAt.Format(time.RFC3339),
			detailsJSON,
		})
	}
}

func parseAuditGroup(raw string) (string, bool) {
	group := strings.TrimSpace(raw)
	if group == "" {
		return "", true
	}
	if group == "admin" {
		return "admin_", true
	}
	return "", false
}

func parseBoundedInt(raw string, fallback int, max int) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("must be non-negative")
	}
	if max > 0 && value > max {
		return max, nil
	}
	return value, nil
}
