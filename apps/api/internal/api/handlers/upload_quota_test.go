package handlers

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/allopze/reform-lab/apps/api/internal/storage"
	"github.com/google/uuid"
)

func TestEnforceCumulativeQuota_RegisteredUser(t *testing.T) {
	userID := uuid.New()
	user := &domain.User{ID: userID}

	h := &UploadHandler{
		Files:                          &fakeQuotaFiles{usedBytes: 40 * 1024 * 1024},
		GuestCumulativeQuotaBytes:      50 * 1024 * 1024,
		RegisteredCumulativeQuotaBytes: 100 * 1024 * 1024,
	}

	// Under quota
	if err := h.enforceCumulativeQuota(context.Background(), user, nil, 10*1024*1024); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Exactly at quota
	if err := h.enforceCumulativeQuota(context.Background(), user, nil, 60*1024*1024); err != nil {
		t.Fatalf("expected no error at limit, got %v", err)
	}

	// Over quota
	err := h.enforceCumulativeQuota(context.Background(), user, nil, 61*1024*1024)
	if err != domain.ErrQuotaExceeded {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestEnforceCumulativeQuota_GuestSession(t *testing.T) {
	guestID := uuid.New()

	h := &UploadHandler{
		Files:                          &fakeQuotaFiles{usedBytes: 45 * 1024 * 1024},
		GuestCumulativeQuotaBytes:      50 * 1024 * 1024,
		RegisteredCumulativeQuotaBytes: 100 * 1024 * 1024,
	}

	// Under quota
	if err := h.enforceCumulativeQuota(context.Background(), nil, &guestID, 4*1024*1024); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Over quota
	err := h.enforceCumulativeQuota(context.Background(), nil, &guestID, 6*1024*1024)
	if err != domain.ErrQuotaExceeded {
		t.Fatalf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestEnforceCumulativeQuota_NoIdentity(t *testing.T) {
	h := &UploadHandler{
		Files:                          &fakeQuotaFiles{usedBytes: 999 * 1024 * 1024},
		GuestCumulativeQuotaBytes:      50 * 1024 * 1024,
		RegisteredCumulativeQuotaBytes: 100 * 1024 * 1024,
	}

	// No user and no guest session — should pass
	if err := h.enforceCumulativeQuota(context.Background(), nil, nil, 100*1024*1024); err != nil {
		t.Fatalf("expected no error for unknown identity, got %v", err)
	}
}

func TestEnforceCumulativeQuota_ZeroQuotaDisabled(t *testing.T) {
	userID := uuid.New()
	user := &domain.User{ID: userID}

	h := &UploadHandler{
		Files:                          &fakeQuotaFiles{usedBytes: 999 * 1024 * 1024},
		GuestCumulativeQuotaBytes:      0,
		RegisteredCumulativeQuotaBytes: 0,
	}

	// Zero quota = disabled, should not enforce
	if err := h.enforceCumulativeQuota(context.Background(), user, nil, 100*1024*1024); err != nil {
		t.Fatalf("expected no error when quota is 0 (disabled), got %v", err)
	}
}

func TestRemainingCumulativeQuota(t *testing.T) {
	userID := uuid.New()
	user := &domain.User{ID: userID}
	h := &UploadHandler{
		Files:                          &fakeQuotaFiles{usedBytes: 40 * 1024 * 1024},
		RegisteredCumulativeQuotaBytes: 100 * 1024 * 1024,
	}

	remaining, err := h.remainingCumulativeQuota(context.Background(), user, nil)
	if err != nil {
		t.Fatalf("remainingCumulativeQuota: %v", err)
	}
	if remaining != 60*1024*1024 {
		t.Fatalf("expected 60 MiB remaining, got %d", remaining)
	}
}

func TestRemainingCumulativeQuotaDisabled(t *testing.T) {
	h := &UploadHandler{
		Files:                          &fakeQuotaFiles{usedBytes: 40 * 1024 * 1024},
		RegisteredCumulativeQuotaBytes: 0,
	}
	user := &domain.User{ID: uuid.New()}

	remaining, err := h.remainingCumulativeQuota(context.Background(), user, nil)
	if err != nil {
		t.Fatalf("remainingCumulativeQuota: %v", err)
	}
	if remaining != -1 {
		t.Fatalf("expected disabled quota marker -1, got %d", remaining)
	}
}

func TestCopyUploadedFileStopsAtStagingLimit(t *testing.T) {
	var dst bytes.Buffer
	size, err := copyUploadedFile(&dst, bytes.NewReader([]byte("abcdef")), 5)
	if !errors.Is(err, errUploadStagingLimitExceeded) {
		t.Fatalf("expected staging limit error, got %v", err)
	}
	if size != 6 {
		t.Fatalf("expected 6 bytes copied before limit detection, got %d", size)
	}
	if dst.String() != "abcdef" {
		t.Fatalf("unexpected staged content %q", dst.String())
	}
}

func TestEnsureUploadDiskHeadroom(t *testing.T) {
	h := &UploadHandler{
		Store: fakeDiskStatsStore{free: storage.MinFreeDiskBytes + 9},
	}

	if err := h.ensureUploadDiskHeadroom(5); !errors.Is(err, storage.ErrInsufficientDisk) {
		t.Fatalf("expected insufficient disk, got %v", err)
	}

	h.Store = fakeDiskStatsStore{free: storage.MinFreeDiskBytes + 10}
	if err := h.ensureUploadDiskHeadroom(5); err != nil {
		t.Fatalf("expected enough disk headroom, got %v", err)
	}
}

// fakeQuotaFiles implements only the methods needed by enforceCumulativeQuota.
type fakeQuotaFiles struct {
	usedBytes int64
	repository.FileRepository
}

func (f *fakeQuotaFiles) CumulativeBytesByUser(_ context.Context, _ uuid.UUID) (int64, error) {
	return f.usedBytes, nil
}

func (f *fakeQuotaFiles) CumulativeBytesByGuestSession(_ context.Context, _ uuid.UUID) (int64, error) {
	return f.usedBytes, nil
}

type fakeDiskStatsStore struct {
	free uint64
	storage.Store
}

func (s fakeDiskStatsStore) DiskStats() (free uint64, total uint64, err error) {
	return s.free, s.free, nil
}

func (s fakeDiskStatsStore) SaveOriginal(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (s fakeDiskStatsStore) GetOriginal(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s fakeDiskStatsStore) OriginalPath(string) string {
	return ""
}

func (s fakeDiskStatsStore) SaveArtifact(context.Context, string, string, io.Reader) (string, error) {
	return "", nil
}

func (s fakeDiskStatsStore) GetArtifact(context.Context, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s fakeDiskStatsStore) GetArtifactByName(string, string) (io.ReadCloser, error) {
	return nil, nil
}

func (s fakeDiskStatsStore) ArtifactPath(string) string {
	return ""
}

func (s fakeDiskStatsStore) CreateTempDir(context.Context, string) (string, error) {
	return "", nil
}

func (s fakeDiskStatsStore) CleanupTemp(context.Context, string) error {
	return nil
}
