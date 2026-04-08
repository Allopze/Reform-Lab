package capabilities

import "github.com/allopze/reform-lab/apps/api/internal/domain"

// Resolve returns the list of capabilities available for a given file.
// It filters by source format, excludes same-format conversions, checks limits,
// and verifies that the required engine binary is present at runtime.
func Resolve(file domain.OriginalFile) []domain.Capability {
	var result []domain.Capability

	for _, cap := range Catalog {
		if !DefaultFlags.Allows(cap) {
			continue
		}

		if !cap.IsSourceSupported(file.DetectedFormat.MIMEType) {
			continue
		}

		// Exclude converting to the same format as the source.
		if cap.TargetFormat == file.DetectedFormat.Extension {
			continue
		}

		// Enforce size limits.
		if file.Size > cap.SizeLimits.MaxInputBytes {
			continue
		}

		// Exclude protected files.
		if file.Metadata.IsProtected {
			continue
		}

		// Exclude capabilities whose engine is not available in this runtime.
		if !DefaultProber.IsAvailable(cap.Engine) {
			continue
		}

		result = append(result, cap)
	}

	return result
}

// IsEligible checks whether a specific capability can be applied to a file.
func IsEligible(file domain.OriginalFile, capabilityID string) (*domain.Capability, error) {
	cap := ByID(capabilityID)
	if cap == nil {
		return nil, domain.ErrCapabilityNotFound
	}

	if !DefaultFlags.Allows(*cap) {
		return nil, domain.ErrCapabilityIneligible
	}

	if !DefaultProber.IsAvailable(cap.Engine) {
		return nil, domain.ErrCapabilityIneligible
	}

	if !cap.IsSourceSupported(file.DetectedFormat.MIMEType) {
		return nil, domain.ErrCapabilityIneligible
	}

	if cap.TargetFormat == file.DetectedFormat.Extension {
		return nil, domain.ErrCapabilityIneligible
	}

	if file.Size > cap.SizeLimits.MaxInputBytes {
		return nil, domain.ErrLimitExceeded
	}

	if file.Metadata.IsProtected {
		return nil, domain.ErrProtectedUnsupported
	}

	return cap, nil
}
