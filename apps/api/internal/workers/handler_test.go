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

func TestOutputArtifactFormatUsesActualExtension(t *testing.T) {
	if got := outputArtifactFormat("/tmp/pages.zip", "jpg"); got != "zip" {
		t.Fatalf("expected zip output format, got %s", got)
	}
	if got := outputArtifactFormat("/tmp/converted", "pdf"); got != "pdf" {
		t.Fatalf("expected fallback format, got %s", got)
	}
}

func TestFormatHelpersCoverNewFormats(t *testing.T) {
	if got := familyForOutputFormat("xlsx"); got != domain.FamilyDocument {
		t.Fatalf("expected xlsx to map to document family, got %s", got)
	}
	if got := familyForOutputFormat("m4a"); got != domain.FamilyAudio {
		t.Fatalf("expected m4a to map to audio family, got %s", got)
	}
	if got := mimeForFormat("opus"); got != "audio/opus" {
		t.Fatalf("expected opus mime, got %s", got)
	}
	if got := mimeForFormat("csv"); got != "text/csv" {
		t.Fatalf("expected csv mime, got %s", got)
	}
}
