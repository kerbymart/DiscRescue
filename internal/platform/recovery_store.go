package platform

import (
	"fmt"

	"discrescue/internal/mapfile"
)

func flushRecoveryStore(store recoveryExtentStore) error {
	if persistence, ok := store.(recoveryDataPersistence); ok {
		return persistence.ForceCheckpoint(CheckpointReasonRecoveryReturn)
	}
	if batched, ok := store.(recoveryBatchedStore); ok {
		if err := batched.Flush(); err != nil {
			return fmt.Errorf("flush recovery map: %w", err)
		}
	}
	return nil
}
func forceRecoveryCheckpoint(store recoveryExtentStore, reason CheckpointReason) error {
	if persistence, ok := store.(recoveryDataPersistence); ok {
		return persistence.ForceCheckpoint(reason)
	}
	return nil
}
func progressExtents(store recoveryExtentStore) []mapfile.Extent {
	if durable, ok := store.(recoveryDurableSnapshot); ok {
		return durable.DurableExtents()
	}
	return store.Extents()
}
