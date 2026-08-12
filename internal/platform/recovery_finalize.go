package platform

import (
	"fmt"

	"discrescue/internal/mapfile"
)

func finalizeUnresolvedRanges(store recoveryExtentStore) error {
	for _, extent := range retryableExtents(store.Extents()) {
		final := extent
		final.State = mapfile.SectorStateMissing
		final.Confidence = mapfile.ConfidenceNone
		if err := store.ApplyExtent(final); err != nil {
			return fmt.Errorf("finalize unreadable range [%d,%d): %w", final.StartLBA, final.EndLBA(), err)
		}
	}
	return nil
}
