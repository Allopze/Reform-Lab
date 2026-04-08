package capabilities

import (
	"sort"
	"strings"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

type FeatureFlags struct {
	disabledCapabilities map[string]struct{}
	disabledEngines      map[string]struct{}
}

type FeatureFlagSnapshot struct {
	DisabledCapabilities []string `json:"disabledCapabilities"`
	DisabledEngines      []string `json:"disabledEngines"`
}

var DefaultFlags = NewFeatureFlags(nil, nil)

func NewFeatureFlags(disabledCapabilities, disabledEngines []string) *FeatureFlags {
	return &FeatureFlags{
		disabledCapabilities: buildFeatureFlagSet(disabledCapabilities),
		disabledEngines:      buildFeatureFlagSet(disabledEngines),
	}
}

func ConfigureFeatureFlags(disabledCapabilities, disabledEngines []string) {
	DefaultFlags = NewFeatureFlags(disabledCapabilities, disabledEngines)
}

func (f *FeatureFlags) Allows(cap domain.Capability) bool {
	return f.IsCapabilityEnabled(cap.ID) && f.IsEngineEnabled(cap.Engine)
}

func (f *FeatureFlags) IsCapabilityEnabled(capabilityID string) bool {
	if f == nil {
		return true
	}
	_, disabled := f.disabledCapabilities[normalizeFeatureFlagValue(capabilityID)]
	return !disabled
}

func (f *FeatureFlags) IsEngineEnabled(engine string) bool {
	if f == nil {
		return true
	}
	_, disabled := f.disabledEngines[normalizeFeatureFlagValue(engine)]
	return !disabled
}

func (f *FeatureFlags) Snapshot() FeatureFlagSnapshot {
	if f == nil {
		return FeatureFlagSnapshot{
			DisabledCapabilities: []string{},
			DisabledEngines:      []string{},
		}
	}

	capabilities := make([]string, 0, len(f.disabledCapabilities))
	for value := range f.disabledCapabilities {
		capabilities = append(capabilities, value)
	}
	engines := make([]string, 0, len(f.disabledEngines))
	for value := range f.disabledEngines {
		engines = append(engines, value)
	}
	sort.Strings(capabilities)
	sort.Strings(engines)

	return FeatureFlagSnapshot{
		DisabledCapabilities: capabilities,
		DisabledEngines:      engines,
	}
}

func EffectiveEngineAvailability() map[string]bool {
	engines := DefaultProber.AvailableEngines()
	for name, available := range engines {
		if available && !DefaultFlags.IsEngineEnabled(name) {
			engines[name] = false
		}
	}
	return engines
}

func buildFeatureFlagSet(values []string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, value := range values {
		normalized := normalizeFeatureFlagValue(value)
		if normalized == "" {
			continue
		}
		result[normalized] = struct{}{}
	}
	return result
}

func normalizeFeatureFlagValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
