package handlers

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/ingestion"
	"github.com/allopze/reform-lab/apps/api/internal/observability"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/security"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

// maxUploadSize is the absolute maximum body size for uploads (500 MB).
const maxUploadSize = 500 * 1024 * 1024

// UploadHandler handles POST /api/files.
type UploadHandler struct {
	Settings repository.SiteSettingRepository
	Store    storage.Store
	Files    repository.FileRepository
	Audit    repository.AuditRepository
	Logger   zerolog.Logger
	Metrics  *observability.Metrics
}

func (h *UploadHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r) // may be nil for anonymous uploads

	policy, err := loadUploadPolicy(r.Context(), h.Settings)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load upload policy")
		return
	}
	effectiveUploadLimit := effectiveUploadLimitBytes(u, policy)

	r.Body = http.MaxBytesReader(w, r.Body, uploadBodyLimitBytes(u, policy))

	file, header, err := r.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(w, http.StatusRequestEntityTooLarge, "file exceeds size limit")
			return
		}
		respondError(w, http.StatusBadRequest, "missing or invalid file field")
		return
	}
	defer file.Close()
	tempFile, err := os.CreateTemp("", "reform-upload-*")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to stage uploaded file")
		return
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
	}()

	// Stream the upload into a temporary file so large bodies stay off-heap.
	size, err := io.Copy(tempFile, file)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(w, http.StatusRequestEntityTooLarge, "file exceeds size limit")
			return
		}
		respondError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}
	if size > effectiveUploadLimit {
		respondError(w, http.StatusRequestEntityTooLarge, "file exceeds size limit")
		return
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to inspect uploaded file")
		return
	}

	// Detect real format from content (never trust extension).
	detected, err := ingestion.DetectFormat(tempFile)
	if err != nil {
		if err == domain.ErrFormatUnsupported {
			respondError(w, http.StatusUnprocessableEntity, "format not supported")
			return
		}
		respondError(w, http.StatusInternalServerError, "format detection failed")
		return
	}

	// Assign internal identity.
	fileID := uuid.New()
	internalName := fileID.String()
	originalName := security.SanitizeFileName(header.Filename)

	// Extract metadata from the staged file before committing it to long-lived storage.
	meta := ingestion.ExtractMetadata(tempPath, detected)

	// Validate file against limits and policies.
	if err := ingestion.ValidateFile(size, detected, meta); err != nil {
		switch err {
		case domain.ErrInvalidCorrupted:
			respondError(w, http.StatusUnprocessableEntity, "file appears empty or corrupted")
		case domain.ErrLimitExceeded:
			respondError(w, http.StatusRequestEntityTooLarge, "file exceeds size limit")
		case domain.ErrProtectedUnsupported:
			respondError(w, http.StatusUnprocessableEntity, "protected or encrypted files not supported")
		default:
			respondError(w, http.StatusUnprocessableEntity, "file validation failed")
		}
		return
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to prepare uploaded file")
		return
	}

	// Persist original file to storage only after validation succeeds.
	storagePath, err := h.Store.SaveOriginal(r.Context(), fileID.String(), tempFile)
	if err != nil {
		h.Logger.Error().Err(err).Str("file_id", fileID.String()).Msg("save original failed")
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	now := time.Now().UTC()
	record := domain.OriginalFile{
		ID:             fileID,
		UserID:         userIDPtr(u),
		InternalName:   internalName,
		OriginalName:   originalName,
		Size:           size,
		DetectedFormat: detected,
		Metadata:       meta,
		UploadedAt:     now,
	}

	if err := h.Files.Create(r.Context(), &record); err != nil {
		// Clean up the stored file to avoid orphan data.
		// storagePath is like <base>/originals/<fileID>/data — remove the parent dir.
		if dir := filepath.Dir(storagePath); dir != "." {
			_ = os.RemoveAll(dir)
		}
		h.Logger.Error().Err(err).Str("file_id", fileID.String()).Msg("persist file record failed")
		respondError(w, http.StatusInternalServerError, "failed to register file")
		return
	}

	// Audit event
	auditDetails := map[string]interface{}{
		"originalName": originalName,
		"mimeType":     detected.MIMEType,
		"size":         size,
	}
	if u != nil {
		auditDetails["userId"] = u.ID.String()
	}
	_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: domain.AuditUpload,
		FileID:    &fileID,
		Details:   auditDetails,
		CreatedAt: now,
	})

	h.Metrics.UploadsTotal.WithLabelValues(string(detected.Family)).Inc()

	respondJSON(w, http.StatusCreated, record)
}
