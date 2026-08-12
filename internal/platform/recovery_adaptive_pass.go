package platform

import (
	"context"
	"fmt"
	"io"

	"discrescue/internal/mapfile"
	"discrescue/internal/recovery"
)

func runAdaptivePass(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	store recoveryExtentStore,
	blockSectors uint64,
	attemptLimit uint16,
	passName string,
	deadlines recovery.ReadDeadlinePolicy,
	report func(recoveryPassProgress),
) error {
	reportRecoveryProgress(report, passName, progressExtents(store), nil)
	ranges := retryableExtents(store.Extents())
	for _, extent := range ranges {
		for lba := extent.StartLBA; lba < extent.EndLBA(); {
			if err := ctx.Err(); err != nil {
				return err
			}
			current, _, ok := mapfile.LookupExtent(store.Extents(), lba)
			if !ok {
				lba++
				continue
			}
			if !isRetryableState(current.State) || current.Attempts >= attemptLimit {
				lba = current.EndLBA()
				continue
			}

			sectorsToRead := blockSectors
			if remaining := current.EndLBA() - lba; remaining < sectorsToRead {
				sectorsToRead = remaining
			}
			if err := attemptDeferredBlock(ctx, source, output, logicalSectorSize, store, lba, sectorsToRead, passName, deadlines, report); err != nil {
				return err
			}
			lba += sectorsToRead
		}
	}
	return nil
}
func attemptDeferredBlock(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	store recoveryExtentStore,
	lba uint64,
	sectorsToRead uint64,
	passName string,
	deadlines recovery.ReadDeadlinePolicy,
	report func(recoveryPassProgress),
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	current, _, ok := mapfile.LookupExtent(store.Extents(), lba)
	if !ok || !isRetryableState(current.State) {
		return nil
	}
	if max := current.EndLBA() - lba; sectorsToRead > max {
		sectorsToRead = max
	}
	if sectorsToRead == 0 {
		return nil
	}

	attempts := current.Attempts + 1
	queued := mapfile.Extent{
		StartLBA:   lba,
		Sectors:    uint32(sectorsToRead),
		State:      mapfile.SectorStateQueued,
		Confidence: mapfile.ConfidenceNone,
		Attempts:   attempts,
	}
	if err := store.ApplyExtent(queued); err != nil {
		return fmt.Errorf("queue deferred range [%d,%d): %w", lba, lba+sectorsToRead, err)
	}

	sectorSize := uint64(logicalSectorSize)
	readSize := int(sectorsToRead * sectorSize)
	buffer := make([]byte, readSize)
	offset := int64(lba * sectorSize)
	n, readErr := readAtWithDeadline(ctx, source, buffer, offset, deadlines.DamagedHard)
	if readSucceeded(n, readSize, readErr) {
		if err := persistRecoveredRead(output, store, buffer, offset, mapfile.Extent{
			StartLBA:   lba,
			Sectors:    uint32(sectorsToRead),
			State:      mapfile.SectorStateReadUnverified,
			Confidence: mapfile.ConfidenceSingleRead,
			Attempts:   attempts,
		}); err != nil {
			return err
		}
		reportRecoveryProgress(report, passName, progressExtents(store), nil)
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	deferred := queued
	if err := fatalRecoveryReadError(readErr); err != nil {
		return fmt.Errorf("read %s range [%d,%d): %w", passName, lba, lba+sectorsToRead, err)
	}
	deferred.State = mapfile.SectorStateIOError
	if err := store.ApplyExtent(deferred); err != nil {
		return fmt.Errorf("restore deferred range [%d,%d): %w", lba, lba+sectorsToRead, err)
	}
	reportRecoveryProgress(report, passName, progressExtents(store), []string{
		fmt.Sprintf("Retry %d: %s", attempts, readFailureDetail("failed", lba, sectorsToRead, n, readSize, readErr)),
		"The range remains deferred until its bounded retry budget is exhausted.",
	})
	return nil
}
