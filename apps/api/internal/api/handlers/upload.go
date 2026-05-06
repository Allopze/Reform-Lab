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

var errUploadStagingLimitExceeded = errors.New("upload staging limit exceeded")

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
	TrustProxyHeaders              bool
}

func (h *UploadHandler) Handle(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r) // may be nil for anonymous uploads
	guestSessionID := currentGuestSessionID(r)
	if u != nil {
		guestSessionID = nil
	}
	if u == nil && guestSessionID == nil {
		guestSessionID = ensureGuestSession(w, r, h.TrustProxyHeaders)
	}

	policy, err := loadUploadPolicy(r.Context(), h.Settings)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to load upload policy")
		return
	}
	effectiveUploadLimit := effectiveUploadLimitBytes(u, policy)
	bodyLimit := uploadBodyLimitBytes(u, policy)

	if err := h.ensureUploadDiskHeadroom(bodyLimit); err != nil {
		if errors.Is(err, storage.ErrInsufficientDisk) {
			respondError(w, http.StatusInsufficientStorage, "server storage is full")
			return
		}
		h.Logger.Error().Err(err).Msg("upload disk headroom check failed")
		respondError(w, http.StatusInternalServerError, "failed to check storage capacity")
		return
	}

	remainingQuota, err := h.remainingCumulativeQuota(r.Context(), u, guestSessionID)
	if err != nil {
		h.Logger.Error().Err(err).Msg("cumulative quota precheck failed")
		respondError(w, http.StatusInternalServerError, "failed to check storage quota")
		return
	}
	if remainingQuota == 0 {
		respondError(w, http.StatusRequestEntityTooLarge, "cumulative storage quota exceeded")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, bodyLimit)

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
	stagingLimit := effectiveUploadLimit
	if remainingQuota > 0 && remainingQuota < stagingLimit {
		stagingLimit = remainingQuota
	}
	size, err := copyUploadedFile(tempFile, file, stagingLimit)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			respondError(w, http.StatusRequestEntityTooLarge, "file exceeds size limit")
			return
		}
		if errors.Is(err, errUploadStagingLimitExceeded) {
			if remainingQuota > 0 && remainingQuota < effectiveUploadLimit {
				respondError(w, http.StatusRequestEntityTooLarge, "cumulative storage quota exceeded")
				return
			}
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
	metadataStart := time.Now()
	meta, err := ingestion.ExtractMetadata(r.Context(), tempPath, detected)
	if h.Metrics != nil {
		h.Metrics.MetadataDuration.WithLabelValues(string(detected.Family)).Observe(time.Since(metadataStart).Seconds())
	}
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
		case domain.ErrImageDimensionsUnknown:
			respondError(w, http.StatusUnprocessableEntity, "image dimensions could not be verified safely")
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

	if err := h.createFileRecord(r.Context(), u, guestSessionID, &record); err != nil {
		// Clean up the stored file to avoid orphan data.
		// storagePath is like <base>/originals/<fileID>/data — remove the parent dir.
		if dir := filepath.Dir(storagePath); dir != "." {
			_ = os.RemoveAll(dir)
		}
		if errors.Is(err, domain.ErrQuotaExceeded) {
			respondError(w, http.StatusRequestEntityTooLarge, "cumulative storage quota exceeded")
			return
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

func copyUploadedFile(dst io.Writer, src io.Reader, maxBytes int64) (int64, error) {
	limited := &io.LimitedReader{R: src, N: maxBytes + 1}
	size, err := io.Copy(dst, limited)
	if err != nil {
		return size, err
	}
	if size > maxBytes {
		return size, errUploadStagingLimitExceeded
	}
	return size, nil
}

// enforceCumulativeQuota checks that adding fileSize bytes would not exceed the
// cumulative disk quota for the given user or guest session.
func (h *UploadHandler) enforceCumulativeQuota(ctx context.Context, u *domain.User, guestSessionID *uuid.UUID, fileSize int64) error {
	remaining, err := h.remainingCumulativeQuota(ctx, u, guestSessionID)
	if err != nil {
		return err
	}
	if remaining >= 0 && fileSize > remaining {
		return domain.ErrQuotaExceeded
	}
	return nil
}

// remainingCumulativeQuota returns remaining bytes for this identity, or -1
// when cumulative quota enforcement is disabled or no stable identity exists.
func (h *UploadHandler) remainingCumulativeQuota(ctx context.Context, u *domain.User, guestSessionID *uuid.UUID) (int64, error) {
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
		return -1, nil
	}

	if err != nil {
		return 0, err
	}

	if quota <= 0 {
		return -1, nil
	}
	remaining := quota - used
	if remaining < 0 {
		return 0, nil
	}
	return remaining, nil
}

type diskStatsStore interface {
	DiskStats() (free uint64, total uint64, err error)
}

func (h *UploadHandler) ensureUploadDiskHeadroom(estimatedBodyBytes int64) error {
	stats, ok := h.Store.(diskStatsStore)
	if !ok || estimatedBodyBytes <= 0 {
		return nil
	}
	free, _, err := stats.DiskStats()
	if err != nil {
		return err
	}
	required := storage.MinFreeDiskBytes + uint64(estimatedBodyBytes)*2
	if free < required {
		return storage.ErrInsufficientDisk
	}
	return nil
}

type quotaFileCreator interface {
	CreateIfUnderQuota(ctx context.Context, f *domain.OriginalFile, quotaBytes int64) error
}

func (h *UploadHandler) createFileRecord(ctx context.Context, u *domain.User, guestSessionID *uuid.UUID, record *domain.OriginalFile) error {
	quota := h.cumulativeQuotaFor(u, guestSessionID)
	if creator, ok := h.Files.(quotaFileCreator); ok {
		return creator.CreateIfUnderQuota(ctx, record, quota)
	}
	return h.Files.Create(ctx, record)
}

func (h *UploadHandler) cumulativeQuotaFor(u *domain.User, guestSessionID *uuid.UUID) int64 {
	if u != nil {
		return h.RegisteredCumulativeQuotaBytes
	}
	if guestSessionID != nil {
		return h.GuestCumulativeQuotaBytes
	}
	return 0
}
