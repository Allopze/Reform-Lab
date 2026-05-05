package handlers

import (
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ArtifactHandler handles GET /api/artifacts/{artifactId}/download.
type ArtifactHandler struct {
	Artifacts repository.ArtifactRepository
	Files     repository.FileRepository
	Store     storage.Store
	Audit     repository.AuditRepository
}

func (h *ArtifactHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r) // may be nil for anonymous users
	guestSessionID := currentGuestSessionID(r)

	artID, err := uuid.Parse(chi.URLParam(r, "artifactId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid artifact ID")
		return
	}

	artifact, err := h.Artifacts.GetByID(r.Context(), artID)
	if err != nil {
		respondError(w, http.StatusNotFound, "artifact not found")
		return
	}
	file, err := h.Files.GetByID(r.Context(), artifact.FileID)
	if err != nil {
		respondError(w, http.StatusNotFound, "file not found")
		return
	}
	if !canAccessResource(u, guestSessionID, file.UserID, file.GuestSessionID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if artifact.ExpiresAt.Before(time.Now().UTC()) {
		respondError(w, http.StatusGone, "artifact expired")
		return
	}

	reader, err := h.Store.GetArtifactByName(artID.String(), artifact.FileName)
	if err != nil {
		// File may have been purged by retention policy between expiry check and read
		respondError(w, http.StatusGone, "artifact file no longer available")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", artifact.MIMEType)
	disposition := mime.FormatMediaType("attachment", map[string]string{
		"filename": artifact.FileName,
	})
	w.Header().Set("Content-Disposition", disposition)
	written, copyErr := io.Copy(w, reader)
	if copyErr != nil {
		return
	}

	if h.Audit != nil {
		details := map[string]interface{}{
			"artifactId": artifact.ID.String(),
			"fileName":   artifact.FileName,
			"bytes":      written,
		}
		if u != nil {
			details["userId"] = u.ID.String()
		}
		if guestSessionID != nil {
			details["guestSessionId"] = guestSessionID.String()
		}
		_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
			ID:        uuid.New(),
			EventType: domain.AuditArtifactDownloaded,
			FileID:    &artifact.FileID,
			JobID:     &artifact.JobID,
			Details:   details,
			CreatedAt: time.Now().UTC(),
		})
	}
}
