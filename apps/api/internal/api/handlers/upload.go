package handlers

import (
	"context"
	"errors"
	"io"
	"mime/multipart"
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
	Settings                       repository.SiteSettingRepository
	Store                          storage.Store
	Files                          repository.FileRepository
	Audit                          repository.AuditRepository
	Logger                         zerolog.Logger
	Metrics                        *observability.Metrics
	GuestCumulativeQuotaBytes      int64
	RegisteredCumulativeQuotaBytes int64
}

func (h *UploadHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r) // may be nil for anonymous uploads
	guestSessionID := currentGuestSessionID(r)
	if u != nil {
		guestSessionID = nil
	}
	if u == nil && guestSessionID == nil {
		guestSessionID = ensureGuestSession(w, r)
	}

	policy, err := loadUploadPolicy(r.Context(), h.Settings)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load upload policy")
		return
	}
	effectiveUploadLimit := effectiveUploadLimitBytes(u, policy)

	r.Body = http.MaxBytesReader(w, r.Body, uploadBodyLimitBytes(u, policy))

	file, originalFileName, err := uploadFilePart(r)
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

	stageID := "upload-" + uuid.NewString()
	tempDir, err := h.Store.CreateTempDir(r.Context(), stageID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to stage uploaded file")
		return
	}
	defer func() {
		_ = h.Store.CleanupTemp(r.Context(), stageID)
	}()

	tempPath := filepath.Join(tempDir, "upload.bin")
	tempFile, err := os.Create(tempPath)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to stage uploaded file")
		return
	}
	defer func() {
		_ = tempFile.Close()
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

	// Check cumulative disk quota for this user or guest session.
	if err := h.enforceCumulativeQuota(r.Context(), u, guestSessionID, size); err != nil {
		if errors.Is(err, domain.ErrQuotaExceeded) {
			respondError(w, http.StatusRequestEntityTooLarge, "cumulative storage quota exceeded")
			return
		}
		h.Logger.Error().Err(err).Msg("cumulative quota check failed")
		respondError(w, http.StatusInternalServerError, "failed to check storage quota")
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
	originalName := security.SanitizeFileName(originalFileName)

	// Extract metadata from the staged file before committing it to long-lived storage.
	meta, err := ingestion.ExtractMetadata(r.Context(), tempPath, detected)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			respondError(w, http.StatusRequestEntityTooLarge, "file is too complex to inspect safely")
			return
		}
		if errors.Is(err, context.Canceled) {
			respondError(w, http.StatusRequestTimeout, "request cancelled during file inspection")
			return
		}
		h.Logger.Warn().Err(err).Str("detected_mime", detected.MIMEType).Msg("metadata extraction failed")
		respondError(w, http.StatusUnprocessableEntity, "file metadata could not be inspected safely")
		return
	}

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
		if errors.Is(err, storage.ErrInsufficientDisk) {
			respondError(w, http.StatusInsufficientStorage, "server storage is full")
			return
		}
		h.Logger.Error().Err(err).Str("file_id", fileID.String()).Msg("save original failed")
		respondError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	now := time.Now().UTC()
	record := domain.OriginalFile{
		ID:             fileID,
		UserID:         userIDPtr(u),
		GuestSessionID: guestSessionID,
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

func uploadFilePart(r *http.Request) (*multipart.Part, string, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, "", err
	}

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return nil, "", errString("missing file field")
		}
		if err != nil {
			return nil, "", err
		}
		if part.FormName() != "file" || part.FileName() == "" {
			part.Close()
			continue
		}
		return part, part.FileName(), nil
	}
}

// enforceCumulativeQuota checks that adding fileSize bytes would not exceed the
// cumulative disk quota for the given user or guest session.
func (h *UploadHandler) enforceCumulativeQuota(ctx context.Context, u *domain.User, guestSessionID *uuid.UUID, fileSize int64) error {
	var used int64
	var quota int64
	var err error

	if u != nil {
		quota = h.RegisteredCumulativeQuotaBytes
		used, err = h.Files.CumulativeBytesByUser(ctx, u.ID)
	} else if guestSessionID != nil {
		quota = h.GuestCumulativeQuotaBytes
		used, err = h.Files.CumulativeBytesByGuestSession(ctx, *guestSessionID)
	} else {
		// No identity — allow the upload; rate-limits still apply.
		return nil
	}

	if err != nil {
		return err
	}

	if quota > 0 && used+fileSize > quota {
		return domain.ErrQuotaExceeded
	}
	return nil
}
