package handlers

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"strconv"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	emailpkg "github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/security"
	"github.com/google/uuid"
)

// SMTPSettingsHandler manages SMTP configuration via admin panel.
type SMTPSettingsHandler struct {
	Email    *emailpkg.Service
	Settings repository.SiteSettingRepository
	Secrets  *security.SecretKeeper
	Audit    repository.AuditRepository
}

type smtpSettingsResponse struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
	UseTLS   bool   `json:"use_tls"`
	Source   string `json:"source"` // "env", "admin", or "none"
}

type smtpSettingsRequest struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
	UseTLS   *bool  `json:"use_tls"`
}

// Get returns the current SMTP config with password masked.
func (h *SMTPSettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	cfg := h.Email.ResolveSMTPConfig(r.Context())

	source := "none"
	if cfg.Host != "" {
		// Check if override exists in site_settings.
		if v, ok, _ := h.Settings.GetValue(r.Context(), emailpkg.SettingSMTPHost); ok && v != "" {
			source = "admin"
		} else {
			source = "env"
		}
	}

	password := ""
	if cfg.Password != "" {
		password = "****"
	}

	respondJSON(w, http.StatusOK, smtpSettingsResponse{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: password,
		From:     cfg.From,
		UseTLS:   cfg.UseTLS,
		Source:   source,
	})
}

// Update persists SMTP settings to site_settings (overriding env defaults).
func (h *SMTPSettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req smtpSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Host != "" {
		if req.Port < 1 || req.Port > 65535 {
			respondError(w, http.StatusBadRequest, "port must be between 1 and 65535")
			return
		}
		if req.From != "" {
			if _, err := mail.ParseAddress(req.From); err != nil {
				respondError(w, http.StatusBadRequest, "invalid from email address")
				return
			}
		}
	}

	now := time.Now().UTC()
	ctx := r.Context()
	encryptedPassword := ""
	if req.Password != "" && req.Password != "****" {
		if h.Secrets == nil {
			respondError(w, http.StatusServiceUnavailable, "secret storage is not configured")
			return
		}
		var err error
		encryptedPassword, err = h.Secrets.Encrypt(req.Password)
		if err != nil {
			respondError(w, http.StatusServiceUnavailable, "secret storage is not configured")
			return
		}
	}

	values := map[string]string{
		emailpkg.SettingSMTPHost: req.Host,
		emailpkg.SettingSMTPPort: strconv.Itoa(req.Port),
		emailpkg.SettingSMTPUser: req.User,
		emailpkg.SettingSMTPFrom: req.From,
	}
	// Only update password if not the mask placeholder.
	if req.Password != "" && req.Password != "****" {
		values[emailpkg.SettingSMTPPassword] = encryptedPassword
	}
	if req.UseTLS != nil {
		tlsVal := "false"
		if *req.UseTLS {
			tlsVal = "true"
		}
		values[emailpkg.SettingSMTPUseTLS] = tlsVal
	}
	if err := h.Settings.UpsertValues(ctx, values, now); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save SMTP settings")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "saved"})

	if h.Audit != nil {
		u := currentUser(r)
		details := map[string]interface{}{"host": req.Host, "port": req.Port, "from": req.From}
		if u != nil {
			details["adminId"] = u.ID.String()
		}
		_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
			ID:        uuid.New(),
			EventType: domain.AuditAdminSMTPUpdated,
			Details:   details,
			CreatedAt: time.Now().UTC(),
		})
	}
}

// Test sends a test email to the authenticated admin's email.
func (h *SMTPSettingsHandler) Test(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	if err := h.Email.SendTestEmail(r.Context(), u.Email); err != nil {
		respondJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Test email sent to " + u.Email,
	})

	if h.Audit != nil {
		details := map[string]interface{}{"recipientEmail": u.Email}
		details["adminId"] = u.ID.String()
		_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
			ID:        uuid.New(),
			EventType: domain.AuditAdminSMTPTest,
			Details:   details,
			CreatedAt: time.Now().UTC(),
		})
	}
}
