package domain

// OperationType classifies what a capability does.
type OperationType string

const (
	OpConvert  OperationType = "convert"
	OpExtract  OperationType = "extract"
	OpCompress OperationType = "compress"
	OpOptimize OperationType = "optimize"
	OpPreview  OperationType = "preview"
)

// SizeLimits defines the maximum input size for a capability.
type SizeLimits struct {
	MaxInputBytes int64 `json:"maxInputBytes"`
}

// ExecutionLimits defines timeouts and resource caps for a capability.
type ExecutionLimits struct {
	TimeoutSeconds int `json:"timeoutSeconds"`
	MaxRetries     int `json:"maxRetries"`
}

// Capability is a declared operation the system can offer for a given format.
type Capability struct {
	ID               string          `json:"id"`
	DisplayName      string          `json:"displayName"`
	SourceFormats    []string        `json:"sourceFormats"`
	OperationType    OperationType   `json:"operationType"`
	TargetFormat     string          `json:"targetFormat"`
	SizeLimits       SizeLimits      `json:"sizeLimits"`
	ExecutionLimits  ExecutionLimits `json:"executionLimits"`
	Engine           string          `json:"engine"`
	ExpectedQuality  string          `json:"expectedQuality"`
	KnownLimitations []string        `json:"knownLimitations,omitempty"`
	Family           FormatFamily    `json:"family"`
}

// IsSourceSupported checks whether a MIME type is in the capability's source list.
func (c Capability) IsSourceSupported(mime string) bool {
	for _, s := range c.SourceFormats {
		if s == mime {
			return true
		}
	}
	return false
}
