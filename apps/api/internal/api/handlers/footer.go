package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/allopze/reform-lab/apps/api/internal/repository"
)

const footerMessageSettingKey = "footer_message"
const DefaultFooterMessage = "© 2026 Reform Lab — Deteccion real de formato y conversion segura"
const footerMessageMaxRunes = 240

type FooterHandler struct {
	Settings repository.SiteSettingRepository
}

type footerMessageRequest struct {
	Message string `json:"message"`
}

func (h *FooterHandler) Get(w http.ResponseWriter, r *http.Request) {
	message, err := h.currentMessage(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load footer message")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": message})
}

func (h *FooterHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req footerMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		respondError(w, http.StatusBadRequest, "footer message is required")
		return
	}
	if utf8.RuneCountInString(message) > footerMessageMaxRunes {
		respondError(w, http.StatusBadRequest, "footer message is too long")
		return
	}

	if err := h.Settings.UpsertValue(r.Context(), footerMessageSettingKey, message, time.Now().UTC()); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to update footer message")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"message": message})
}

func (h *FooterHandler) currentMessage(ctx context.Context) (string, error) {
	message, ok, err := h.Settings.GetValue(ctx, footerMessageSettingKey)
	if err != nil {
		return "", err
	}
	if !ok {
		return DefaultFooterMessage, nil
	}

	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return DefaultFooterMessage, nil
	}
	return trimmed, nil
}
