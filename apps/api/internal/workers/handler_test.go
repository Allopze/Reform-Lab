package workers

import (
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

func TestArtifactTTLUsesFamilyOverride(t *testing.T) {
	h := &Handler{
		ArtifactTTL: 24 * time.Hour,
		ArtifactTTLByFamily: map[domain.FormatFamily]time.Duration{
			domain.FamilyPDF:   48 * time.Hour,
			domain.FamilyImage: 12 * time.Hour,
		},
	}

	if got := h.artifactTTL("png"); got != 12*time.Hour {
		t.Fatalf("expected image ttl 12h, got %v", got)
	}
	if got := h.artifactTTL("pdf"); got != 48*time.Hour {
		t.Fatalf("expected pdf ttl 48h, got %v", got)
	}
	if got := h.artifactTTL("mp3"); got != 24*time.Hour {
		t.Fatalf("expected fallback ttl 24h, got %v", got)
	}
}
