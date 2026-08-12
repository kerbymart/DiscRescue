package platform

import (
	"context"
	"fmt"
	"io"

	"discrescue/internal/mapfile"
	"discrescue/internal/recovery"
)

func runFastAcquisitionPass(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	capacitySectors uint64,
	store recoveryExtentStore,
	blockSectors uint32,
	deadlines recovery.ReadDeadlinePolicy,
	report func(recoveryPassProgress),
) error {
	if blockSectors == 0 {
		return fmt.Errorf("run fast acquisition pass: invalid policy limits")
	}
	sectorSize := uint64(logicalSectorSize)
	buffer := make([]byte, uint64(blockSectors)*sectorSize)

	reportRecoveryProgress(report, "Fast acquisition", progressExtents(store), nil)
	for lba := uint64(0); lba < capacitySectors; {
		if err := ctx.Err(); err != nil {
			return err
		}

		extents := store.Extents()
		if extent, _, ok := mapfile.LookupExtent(extents, lba); ok {
			lba = extent.EndLBA()
			continue
		}

		sectorsToRead := uint64(blockSectors)
		if remaining := capacitySectors - lba; remaining < sectorsToRead {
			sectorsToRead = remaining
		}
		for _, extent := range extents {
			if extent.StartLBA > lba && extent.StartLBA-lba < sectorsToRead {
				sectorsToRead = extent.StartLBA - lba
				break
			}
		}
		if sectorsToRead == 0 {
			continue
		}

		readSize := int(sectorsToRead * sectorSize)
		offset := int64(lba * sectorSize)
		n, readErr := readAtWithDeadline(ctx, source, buffer[:readSize], offset, deadlines.HealthyHard)
		if readSucceeded(n, readSize, readErr) {
			if err := persistRecoveredRead(output, store, buffer[:readSize], offset, mapfile.Extent{
				StartLBA:   lba,
				Sectors:    uint32(sectorsToRead),
				State:      mapfile.SectorStateReadUnverified,
				Confidence: mapfile.ConfidenceSingleRead,
				Attempts:   1,
			}); err != nil {
				return err
			}
			reportRecoveryProgress(report, "Fast acquisition", progressExtents(store), nil)
			lba += sectorsToRead
			continue
		}

		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fatalRecoveryReadError(readErr); err != nil {
			return fmt.Errorf("read fast acquisition range [%d,%d): %w", lba, lba+sectorsToRead, err)
		}
		deferred := mapfile.Extent{
			StartLBA:   lba,
			Sectors:    uint32(sectorsToRead),
			State:      mapfile.SectorStateIOError,
			Confidence: mapfile.ConfidenceNone,
			Attempts:   1,
		}
		if err := store.ApplyExtent(deferred); err != nil {
			return fmt.Errorf("defer failed range [%d,%d): %w", lba, lba+sectorsToRead, err)
		}
		reportRecoveryProgress(report, "Fast acquisition", progressExtents(store), []string{
			readFailureDetail("Deferred", lba, sectorsToRead, n, readSize, readErr),
			"Continuing forward; smaller bounded reads will revisit this range later.",
		})
		lba += sectorsToRead
	}
	return nil
}
