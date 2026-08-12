package platform

import "discrescue/internal/mapfile"

func reportRecoveryProgress(report func(recoveryPassProgress), pass string, extents []mapfile.Extent, issue []string) {
	if report == nil {
		return
	}
	scanned, recovered, deferred, unreadable := summarizeRecoveryExtentStates(extents)
	report(recoveryPassProgress{
		Pass:              pass,
		ScannedSectors:    scanned,
		RecoveredSectors:  recovered,
		DeferredSectors:   deferred,
		UnreadableSectors: unreadable,
		LastIssue:         append([]string(nil), issue...),
	})
}
func summarizeRecoveryExtentStates(extents []mapfile.Extent) (scanned uint64, recovered uint64, deferred uint64, unreadable uint64) {
	for _, extent := range extents {
		sectors := uint64(extent.Sectors)
		scanned += sectors
		if recoveryStateHasData(extent.State) {
			recovered += sectors
			continue
		}
		switch extent.State {
		case mapfile.SectorStateUnknown, mapfile.SectorStateQueued, mapfile.SectorStateIOError:
			deferred += sectors
		case mapfile.SectorStateMissing:
			unreadable += sectors
		}
	}
	return scanned, recovered, deferred, unreadable
}
func summarizeRecoveryExtents(extents []mapfile.Extent) (scanned uint64, recovered uint64, unresolved uint64) {
	scanned, recovered, deferred, unreadable := summarizeRecoveryExtentStates(extents)
	return scanned, recovered, deferred + unreadable
}
func unresolvedSectorCount(extents []mapfile.Extent) uint64 {
	_, _, unresolved := summarizeRecoveryExtents(extents)
	return unresolved
}
func recoveryStateHasData(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateReadUnverified,
		mapfile.SectorStateVerified,
		mapfile.SectorStateChecksumError,
		mapfile.SectorStateConflicting,
		mapfile.SectorStateReconstructed:
		return true
	default:
		return false
	}
}
