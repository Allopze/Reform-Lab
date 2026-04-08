package workers

import (
	"context"
	"fmt"
)

// Engine executes a conversion and produces an output file.
type Engine interface {
	// Execute runs the conversion from inputPath, writing results to outputDir.
	// Returns the path to the primary output file.
	Execute(ctx context.Context, inputPath string, outputDir string, outputFormat string) (outputPath string, err error)
}

// Registry maps capability IDs to their responsible engine.
type Registry struct {
	engines map[string]Engine
}

// NewRegistry creates an empty engine registry.
func NewRegistry() *Registry {
	return &Registry{engines: make(map[string]Engine)}
}

// Register associates a capability ID with an engine.
func (r *Registry) Register(capabilityID string, engine Engine) {
	r.engines[capabilityID] = engine
}

// Get returns the engine for a capability, or an error if not found.
func (r *Registry) Get(capabilityID string) (Engine, error) {
	e, ok := r.engines[capabilityID]
	if !ok {
		return nil, fmt.Errorf("no engine registered for capability %q", capabilityID)
	}
	return e, nil
}
