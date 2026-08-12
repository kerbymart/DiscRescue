package platform

import (
	"discrescue/internal/mapfile"
	"discrescue/internal/recovery"
)

func retryPolicyWithCurrentAttempts(policy recovery.RecoveryPolicy, extents []mapfile.Extent) recovery.RecoveryPolicy {
	var offset uint16
	for _, extent := range extents {
		if isRetryableState(extent.State) && extent.Attempts > offset {
			offset = extent.Attempts
		}
	}
	if offset == 0 {
		return policy
	}
	policy.Trim.AttemptsLimit = addRetryAttemptOffset(policy.Trim.AttemptsLimit, offset)
	for i := range policy.Adaptive {
		policy.Adaptive[i].AttemptsLimit = addRetryAttemptOffset(policy.Adaptive[i].AttemptsLimit, offset)
	}
	policy.Targeted.AttemptsLimit = addRetryAttemptOffset(policy.Targeted.AttemptsLimit, offset)
	return policy
}
func addRetryAttemptOffset(limit, offset uint16) uint16 {
	if ^uint16(0)-limit < offset {
		return ^uint16(0)
	}
	return limit + offset
}
func retryableExtents(extents []mapfile.Extent) []mapfile.Extent {
	result := make([]mapfile.Extent, 0)
	for _, extent := range extents {
		if isRetryableState(extent.State) {
			result = append(result, extent)
		}
	}
	return result
}
func isRetryableState(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateUnknown,
		mapfile.SectorStateQueued,
		mapfile.SectorStateIOError,
		mapfile.SectorStateMissing:
		return true
	default:
		return false
	}
}
