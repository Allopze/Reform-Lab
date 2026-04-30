package workers

import (
	"slices"
	"testing"

	"github.com/allopze/reform-lab/apps/api/internal/capabilities"
)

func TestDefaultRegistryCoversCapabilityCatalog(t *testing.T) {
	registryIDs := BuildDefaultRegistry().IDs()

	for _, capability := range capabilities.Catalog {
		if !slices.Contains(registryIDs, capability.ID) {
			t.Fatalf("capability %q is missing from the default worker registry", capability.ID)
		}
	}
}
