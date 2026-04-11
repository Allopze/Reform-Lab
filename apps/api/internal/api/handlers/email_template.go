package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmltmpl "html/template"
	"net/http"
	texttmpl "text/template"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	emailpkg "github.com/allopze/reform-lab/apps/api/internal/email"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/go-chi/chi/v5"
)

const emailTemplateBodyMaxBytes = 100 * 1024 // 100 KB

// EmailTemplateHandler manages email templates via admin panel.
type EmailTemplateHandler struct {
	Email     *emailpkg.Service
	Templates repository.EmailTemplateRepository
}

type emailTemplateRequest struct {
	Subject  string `json:"subject"`
	BodyHTML string `json:"body_html"`
}

// List returns all email templates.
func (h *EmailTemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	templates, err := h.Templates.ListAll(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list templates")
		return
	}
	if templates == nil {
		templates = []domain.EmailTemplate{}
	}
	respondJSON(w, http.StatusOK, templates)
}

// Get returns a single template by key.
func (h *EmailTemplateHandler) Get(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "template key is required")
		return
	}

	tmpl, err := h.Templates.GetByKey(r.Context(), key)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load template")
		return
	}
	if tmpl == nil {
		respondError(w, http.StatusNotFound, "template not found")
		return
	}

	respondJSON(w, http.StatusOK, tmpl)
}

// Update modifies the subject and body of an existing template.
func (h *EmailTemplateHandler) Update(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "template key is required")
		return
	}

	var req emailTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Subject == "" {
		respondError(w, http.StatusBadRequest, "subject is required")
		return
	}
	if req.BodyHTML == "" {
		respondError(w, http.StatusBadRequest, "body_html is required")
		return
	}
	if len(req.BodyHTML) > emailTemplateBodyMaxBytes {
		respondError(w, http.StatusBadRequest, "body_html exceeds maximum size (100 KB)")
		return
	}

	// Validate that subject and body are valid Go templates.
	if _, err := texttmpl.New("subject").Parse(req.Subject); err != nil {
		respondError(w, http.StatusBadRequest, "subject contains invalid template syntax")
		return
	}
	if _, err := htmltmpl.New("body").Parse(req.BodyHTML); err != nil {
		respondError(w, http.StatusBadRequest, "body_html contains invalid template syntax")
		return
	}

	// Check template exists before updating.
	existing, err := h.Templates.GetByKey(r.Context(), key)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check template")
		return
	}
	if existing == nil {
		respondError(w, http.StatusNotFound, "template not found")
		return
	}

	tmpl := &domain.EmailTemplate{
		Key:       key,
		Subject:   req.Subject,
		BodyHTML:  req.BodyHTML,
		UpdatedAt: time.Now().UTC(),
	}
	if err := h.Templates.Upsert(r.Context(), tmpl); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to save template")
		return
	}

	respondJSON(w, http.StatusOK, tmpl)
}

// Preview renders a template with example data and returns the HTML.
// If the request includes subject/body_html in the body, it renders
// those directly (without saving), allowing draft previews.
func (h *EmailTemplateHandler) Preview(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if key == "" {
		respondError(w, http.StatusBadRequest, "template key is required")
		return
	}

	exampleVars := map[string]string{
		"Name":         "Usuario de Ejemplo",
		"Email":        "ejemplo@reforma.com",
		"AppName":      "Reform Lab",
		"AppURL":       "https://reformlab.example.com",
		"Year":         time.Now().Format("2006"),
		"FileName":     "presentacion.pptx",
		"OutputFormat": "PDF",
		"ErrorMessage": "formato de entrada no soportado",
		"ResetURL":     "https://reformlab.example.com/reset?token=abc123",
	}

	// If body contains subject+body_html, render from request (draft preview).
	var req emailTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.BodyHTML != "" {
		msg, err := renderDraft(req.Subject, req.BodyHTML, exampleVars)
		if err != nil {
			respondError(w, http.StatusBadRequest, "template render error: "+err.Error())
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{
			"subject": msg.Subject,
			"html":    msg.BodyHTML,
		})
		return
	}

	// Fallback: render from saved template in DB.
	msg, err := h.Email.RenderTemplate(r.Context(), key, exampleVars)
	if err != nil {
		respondError(w, http.StatusBadRequest, "template render error: "+err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"subject": msg.Subject,
		"html":    msg.BodyHTML,
	})
}

// renderDraft renders subject + body with example vars without touching the DB.
func renderDraft(subject, bodyHTML string, vars map[string]string) (*domain.EmailMessage, error) {
	st, err := texttmpl.New("subject").Parse(subject)
	if err != nil {
		return nil, fmt.Errorf("parse subject: %w", err)
	}
	var sb bytes.Buffer
	if err := st.Execute(&sb, vars); err != nil {
		return nil, fmt.Errorf("render subject: %w", err)
	}

	bt, err := htmltmpl.New("body").Parse(bodyHTML)
	if err != nil {
		return nil, fmt.Errorf("parse body: %w", err)
	}
	var bb bytes.Buffer
	if err := bt.Execute(&bb, vars); err != nil {
		return nil, fmt.Errorf("render body: %w", err)
	}

	return &domain.EmailMessage{
		Subject:  sb.String(),
		BodyHTML: bb.String(),
	}, nil
}
