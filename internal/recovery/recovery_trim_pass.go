package recovery

import (
	"context"
	"io"

	"discrescue/internal/mapfile"
)

func runTrimPass(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	store recoveryExtentStore,
	attemptLimit uint16,
	deadlines ReadDeadlinePolicy,
	report func(recoveryPassProgress),
	classify ReadErrorClassifier,
) error {
	reportRecoveryProgress(report, "Trimming deferred ranges", progressExtents(store), nil)
	ranges := retryableExtents(store.Extents())
	for _, extent := range ranges {
		if extent.Sectors <= 1 {
			continue
		}
		edges := []uint64{extent.StartLBA, extent.EndLBA() - 1}
		for _, lba := range edges {
			if err := ctx.Err(); err != nil {
				return err
			}
			current, _, ok := mapfile.LookupExtent(store.Extents(), lba)
			if !ok || !isRetryableState(current.State) || current.Attempts >= attemptLimit {
				continue
			}
			if err := attemptDeferredBlock(ctx, source, output, logicalSectorSize, store, lba, 1, "Trimming deferred ranges", deadlines, report, classify); err != nil {
				return err
			}
		}
	}
	return nil
}
