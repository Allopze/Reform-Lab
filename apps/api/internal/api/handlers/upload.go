package handlers

import (
	"bytes"
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
	Store   storage.Store
	Files   repository.FileRepository
	Audit   repository.AuditRepository
	Logger  zerolog.Logger
	Metrics *observability.Metrics
}

func (h *UploadHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	if u == nil {
		respondError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "missing or invalid file field")
		return
	}
	defer file.Close()

	// Read file into buffer for detection (mimetype needs a reader).
	var buf bytes.Buffer
	size, err := io.Copy(&buf, file)
	if err != nil {
		respondError(w, http.StatusBadRequest, "failed to read uploaded file")
		return
	}

	// Detect real format from content (never trust extension).
	detected, err := ingestion.DetectFormat(bytes.NewReader(buf.Bytes()))
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

	// Persist original file to storage.
	storagePath, err := h.Store.SaveOriginal(r.Context(), fileID.String(), bytes.NewReader(buf.Bytes()))
	if err != nil {
		h.Logger.Error().Err(err).Str("file_id", fileID.String()).Msg("save original failed")
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	// Extract metadata from the stored file.
	meta := ingestion.ExtractMetadata(storagePath, detected)

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

	now := time.Now().UTC()
	record := domain.OriginalFile{
		ID:             fileID,
		UserID:         &u.ID,
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
	_ = h.Audit.Create(r.Context(), &domain.AuditEvent{
		ID:        uuid.New(),
		EventType: domain.AuditUpload,
		FileID:    &fileID,
		Details: map[string]interface{}{
			"userId":       u.ID.String(),
			"originalName": originalName,
			"mimeType":     detected.MIMEType,
			"size":         size,
		},
		CreatedAt: now,
	})

	h.Metrics.UploadsTotal.WithLabelValues(string(detected.Family)).Inc()

	respondJSON(w, http.StatusCreated, record)
}
