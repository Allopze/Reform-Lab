package handlers

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"strconv"
	"time"

	emailpkg "github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/security"
)

// SMTPSettingsHandler manages SMTP configuration via admin panel.
type SMTPSettingsHandler struct {
	Email    *emailpkg.Service
	Settings repository.SiteSettingRepository
	Secrets  *security.SecretKeeper
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

	if err := h.Settings.UpsertValue(ctx, emailpkg.SettingSMTPHost, req.Host, now); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save SMTP host")
		return
	}
	if err := h.Settings.UpsertValue(ctx, emailpkg.SettingSMTPPort, strconv.Itoa(req.Port), now); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save SMTP port")
		return
	}
	if err := h.Settings.UpsertValue(ctx, emailpkg.SettingSMTPUser, req.User, now); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save SMTP user")
		return
	}
	// Only update password if not the mask placeholder.
	if req.Password != "" && req.Password != "****" {
		encryptedPassword, err := h.Secrets.Encrypt(req.Password)
		if err != nil {
			respondError(w, http.StatusServiceUnavailable, "secret storage is not configured")
			return
		}
		if err := h.Settings.UpsertValue(ctx, emailpkg.SettingSMTPPassword, encryptedPassword, now); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save SMTP password")
			return
		}
	}
	if err := h.Settings.UpsertValue(ctx, emailpkg.SettingSMTPFrom, req.From, now); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save SMTP from")
		return
	}
	if req.UseTLS != nil {
		tlsVal := "false"
		if *req.UseTLS {
			tlsVal = "true"
		}
		if err := h.Settings.UpsertValue(ctx, emailpkg.SettingSMTPUseTLS, tlsVal, now); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to save SMTP TLS setting")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "saved"})
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
}
