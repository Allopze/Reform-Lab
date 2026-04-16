package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

const guestUploadMaxBytesSettingKey = "guest_upload_max_bytes"
const registeredUploadMaxBytesSettingKey = "registered_upload_max_bytes"
const defaultGuestUploadMaxBytes int64 = 10 * 1024 * 1024
const defaultRegisteredUploadMaxBytes int64 = 100 * 1024 * 1024
const minConfiguredUploadLimitBytes int64 = 1 * 1024 * 1024
const maxMultipartBodyOverhead int64 = 1 * 1024 * 1024

type uploadPolicy struct {
	GuestMaxBytes      int64
	RegisteredMaxBytes int64
}

type uploadPolicyResponse struct {
	GuestMaxBytes        int64  `json:"guestMaxBytes"`
	RegisteredMaxBytes   int64  `json:"registeredMaxBytes"`
	EffectiveMaxBytes    int64  `json:"effectiveMaxBytes"`
	ViewerType           string `json:"viewerType"`
	AbsoluteMaxBytes     int64  `json:"absoluteMaxBytes"`
	CumulativeQuotaBytes int64  `json:"cumulativeQuotaBytes"`
	CumulativeUsedBytes  int64  `json:"cumulativeUsedBytes"`
}

type updateUploadPolicyRequest struct {
	GuestMaxBytes      int64 `json:"guestMaxBytes"`
	RegisteredMaxBytes int64 `json:"registeredMaxBytes"`
}

type UploadPolicyHandler struct {
	Settings                       repository.SiteSettingRepository
	Files                          repository.FileRepository
	Audit                          repository.AuditRepository
	GuestCumulativeQuotaBytes      int64
	RegisteredCumulativeQuotaBytes int64
}

func (h *UploadPolicyHandler) Get(w http.ResponseWriter, r *http.Request) {
	policy, err := loadUploadPolicy(r.Context(), h.Settings)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load upload policy")
		return
	}

	u := currentUser(r)
	guestSessionID := currentGuestSessionID(r)
	quota, used := h.cumulativeQuotaInfo(r.Context(), u, guestSessionID)

	resp := buildUploadPolicyResponse(policy, u)
	resp.CumulativeQuotaBytes = quota
	resp.CumulativeUsedBytes = used
	respondJSON(w, http.StatusOK, resp)
}

func (h *UploadPolicyHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateUploadPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	guestMaxBytes, err := normalizeConfiguredUploadLimit(req.GuestMaxBytes)
	if err != nil {
		respondError(w, http.StatusBadRequest, "guest upload limit must be between 1 MB and 500 MB")
		return
	}
	registeredMaxBytes, err := normalizeConfiguredUploadLimit(req.RegisteredMaxBytes)
	if err != nil {
		respondError(w, http.StatusBadRequest, "registered upload limit must be between 1 MB and 500 MB")
		return
	}

	now := time.Now().UTC()
	if err := h.Settings.UpsertValue(r.Context(), guestUploadMaxBytesSettingKey, strconv.FormatInt(guestMaxBytes, 10), now); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update upload policy")
		return
	}
	if err := h.Settings.UpsertValue(r.Context(), registeredUploadMaxBytesSettingKey, strconv.FormatInt(registeredMaxBytes, 10), now); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update upload policy")
		return
	}

	respondJSON(w, http.StatusOK, buildUploadPolicyResponse(uploadPolicy{
		GuestMaxBytes:      guestMaxBytes,
		RegisteredMaxBytes: registeredMaxBytes,
	}, currentUser(r)))

	if h.Audit != nil {
		u := currentUser(r)
		details := map[string]interface{}{
			"guestMaxBytes":      guestMaxBytes,
			"registeredMaxBytes": registeredMaxBytes,
		}
		if u != nil {
			details["adminId"] = u.ID.String()
		}
		_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
			ID:        uuid.New(),
			EventType: domain.AuditAdminUploadPolicy,
			Details:   details,
			CreatedAt: now,
		})
	}
}

func loadUploadPolicy(ctx context.Context, settings repository.SiteSettingRepository) (uploadPolicy, error) {
	guestMaxBytes, err := loadConfiguredUploadLimit(ctx, settings, guestUploadMaxBytesSettingKey, defaultGuestUploadMaxBytes)
	if err != nil {
		return uploadPolicy{}, err
	}
	registeredMaxBytes, err := loadConfiguredUploadLimit(ctx, settings, registeredUploadMaxBytesSettingKey, defaultRegisteredUploadMaxBytes)
	if err != nil {
		return uploadPolicy{}, err
	}
	return uploadPolicy{
		GuestMaxBytes:      guestMaxBytes,
		RegisteredMaxBytes: registeredMaxBytes,
	}, nil
}

func loadConfiguredUploadLimit(ctx context.Context, settings repository.SiteSettingRepository, key string, fallback int64) (int64, error) {
	value, ok, err := settings.GetValue(ctx, key)
	if err != nil {
		return 0, err
	}
	if !ok {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback, nil
	}
	normalized, err := normalizeConfiguredUploadLimit(parsed)
	if err != nil {
		return fallback, nil
	}
	return normalized, nil
}

func normalizeConfiguredUploadLimit(value int64) (int64, error) {
	if value < minConfiguredUploadLimitBytes || value > maxUploadSize {
		return 0, domain.ErrLimitExceeded
	}
	return value, nil
}

func buildUploadPolicyResponse(policy uploadPolicy, user *domain.User) uploadPolicyResponse {
	viewerType := "guest"
	if user != nil {
		viewerType = "registered"
	}

	return uploadPolicyResponse{
		GuestMaxBytes:      policy.GuestMaxBytes,
		RegisteredMaxBytes: policy.RegisteredMaxBytes,
		EffectiveMaxBytes:  effectiveUploadLimitBytes(user, policy),
		ViewerType:         viewerType,
		AbsoluteMaxBytes:   maxUploadSize,
	}
}

func effectiveUploadLimitBytes(user *domain.User, policy uploadPolicy) int64 {
	if user != nil {
		return policy.RegisteredMaxBytes
	}
	return policy.GuestMaxBytes
}

func uploadBodyLimitBytes(user *domain.User, policy uploadPolicy) int64 {
	effective := effectiveUploadLimitBytes(user, policy)
	if effective >= maxUploadSize {
		return maxUploadSize
	}
	bodyLimit := effective + maxMultipartBodyOverhead
	if bodyLimit > maxUploadSize {
		return maxUploadSize
	}
	return bodyLimit
}

// cumulativeQuotaInfo returns the applicable cumulative quota and the current
// usage for the given user or guest session.  On errors it returns (0, 0) so
// the caller still responds without crashing.
func (h *UploadPolicyHandler) cumulativeQuotaInfo(ctx context.Context, u *domain.User, guestSessionID *uuid.UUID) (quota int64, used int64) {
	if h.Files == nil {
		return 0, 0
	}
	if u != nil {
		quota = h.RegisteredCumulativeQuotaBytes
		used, _ = h.Files.CumulativeBytesByUser(ctx, u.ID)
		return quota, used
	}
	if guestSessionID != nil {
		quota = h.GuestCumulativeQuotaBytes
		used, _ = h.Files.CumulativeBytesByGuestSession(ctx, *guestSessionID)
		return quota, used
	}
	return h.GuestCumulativeQuotaBytes, 0
}
