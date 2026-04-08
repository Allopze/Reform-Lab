package handlers

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ArtifactHandler handles GET /api/artifacts/{artifactId}/download.
type ArtifactHandler struct {
	Artifacts repository.ArtifactRepository
	Store     storage.Store
}

func (h *ArtifactHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

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
	if !canAccessOwner(u, artifact.UserID) {
		respondError(w, http.StatusForbidden, "forbidden")
		return
	}
	if artifact.ExpiresAt.Before(time.Now().UTC()) {
		respondError(w, http.StatusGone, "artifact expired")
		return
	}

	reader, err := h.Store.GetArtifactByName(artID.String(), artifact.FileName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to retrieve artifact file")
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", artifact.MIMEType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, artifact.FileName))
	io.Copy(w, reader)
}
