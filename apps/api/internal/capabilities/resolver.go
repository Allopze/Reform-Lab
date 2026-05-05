package capabilities

import (
	"sort"

	"github.com/allopze/reform-lab/apps/api/internal/domain"
)

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

		if rejectsSameFormat(file, cap) {
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

	sortCapabilities(result)

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

	if rejectsSameFormat(file, *cap) {
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

func rejectsSameFormat(file domain.OriginalFile, cap domain.Capability) bool {
	if cap.OperationType != domain.OpConvert {
		return false
	}

	// Allow video re-encoding (e.g., MP4→MP4 with different codec/quality)
	if cap.Family == domain.FamilyVideo {
		return false
	}

	return cap.TargetFormat == file.DetectedFormat.Extension
}

func sortCapabilities(capabilities []domain.Capability) {
	sort.Slice(capabilities, func(i, j int) bool {
		if capabilities[i].PresentationOrder != capabilities[j].PresentationOrder {
			return capabilities[i].PresentationOrder < capabilities[j].PresentationOrder
		}
		return capabilities[i].ID < capabilities[j].ID
	})
}
