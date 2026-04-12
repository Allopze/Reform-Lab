package handlers

import (
	"context"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
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
