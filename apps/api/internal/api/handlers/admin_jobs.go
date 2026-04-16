package handlers

import (
	"net/http"
	"strconv"

	"github.com/allopze/reform-lab/apps/api/internal/repository"
)

type AdminJobsHandler struct {
	Jobs repository.JobRepository
}

func (h *AdminJobsHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	filter := repository.AdminJobFilter{
		Status:       q.Get("status"),
		CapabilityID: q.Get("capability"),
		Search:       q.Get("q"),
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
