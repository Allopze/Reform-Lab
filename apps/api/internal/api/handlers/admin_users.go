package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AdminUsersHandler struct {
	Users repository.UserRepository
	Audit repository.AuditRepository
}

type adminUserResponse struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Email     string          `json:"email"`
	Team      string          `json:"team,omitempty"`
	Role      domain.UserRole `json:"role"`
	CreatedAt string          `json:"createdAt"`
}

type adminUsersPageResponse struct {
	Users []adminUserResponse `json:"users"`
	Total int                 `json:"total"`
}

func (h *AdminUsersHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	role := q.Get("role")
	if role != "" && role != string(domain.RoleAdmin) && role != string(domain.RoleUser) {
		respondError(w, http.StatusBadRequest, "role must be 'admin' or 'user'")
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	page, err := h.Users.ListForAdmin(r.Context(), repository.AdminUserFilter{
		Search: q.Get("q"),
		Role:   role,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}

	result := make([]adminUserResponse, 0, len(page.Users))
	for _, u := range page.Users {
		result = append(result, adminUserResponse{
			ID:        u.ID.String(),
			Name:      u.Name,
			Email:     u.Email,
			Team:      u.Team,
			Role:      u.Role,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}

	respondJSON(w, http.StatusOK, adminUsersPageResponse{Users: result, Total: page.Total})
}

type updateRoleRequest struct {
	Role domain.UserRole `json:"role"`
}

func (h *AdminUsersHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	targetID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Role != domain.RoleAdmin && req.Role != domain.RoleUser {
		respondError(w, http.StatusBadRequest, "role must be 'admin' or 'user'")
		return
	}

	// Prevent demoting yourself.
	caller := currentUser(r)
	if caller != nil && caller.ID == targetID && req.Role != domain.RoleAdmin {
		respondError(w, http.StatusBadRequest, "cannot demote yourself")
		return
	}

	if err := h.Users.UpdateRole(r.Context(), targetID, req.Role); err != nil {
		if err == domain.ErrUserNotFound {
			respondError(w, http.StatusNotFound, "user not found")
			return
		}
		respondError(w, http.StatusInternalServerError, "failed to update role")
		return
	}

	now := time.Now().UTC()
	adminID := ""
	if caller != nil {
		adminID = caller.ID.String()
	}
	_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: domain.AuditAdminRoleChanged,
		Details: map[string]interface{}{
			"targetUserId": targetID.String(),
			"newRole":      string(req.Role),
			"adminId":      adminID,
		},
		CreatedAt: now,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
