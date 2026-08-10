package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"discrescue/internal/mapfile"
	"discrescue/internal/recovery"
)

const (
	fastPassSectors     = uint64(64)
	maxTargetedAttempts = uint16(6)
)

type recoveryExtentStore interface {
	Extents() []mapfile.Extent
	ApplyExtent(mapfile.Extent) error
}

type recoveryBatchedStore interface {
	StageExtent(mapfile.Extent) error
	Flush() error
	PendingBytes() uint64
}

type recoveryDurableSnapshot interface {
	DurableExtents() []mapfile.Extent
}

type recoveryDataPersistence interface {
	recoveryExtentStore
	PersistRecovered(data []byte, offset int64, extent mapfile.Extent) error
	ForceCheckpoint(CheckpointReason) error
}

type recoverySyncWriter interface {
	io.WriterAt
	Sync() error
}

type recoveryPassProgress struct {
	Pass              string
	ScannedSectors    uint64
	RecoveredSectors  uint64
	DeferredSectors   uint64
	UnreadableSectors uint64
	LastIssue         []string
}

func runPassBasedRecovery(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	capacitySectors uint64,
	store recoveryExtentStore,
	report func(recoveryPassProgress),
) (err error) {
	policy, policyErr := recovery.PolicyForMethod(recovery.RecoveryMethodBalanced)
	if policyErr != nil {
		return policyErr
	}
	return runPassBasedRecoveryWithPolicy(ctx, source, output, logicalSectorSize, capacitySectors, store, policy, report)
}

func runPassBasedRecoveryWithPolicy(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	capacitySectors uint64,
	store recoveryExtentStore,
	policy recovery.RecoveryPolicy,
	report func(recoveryPassProgress),
) (err error) {
	if policyErr := policy.Validate(); policyErr != nil {
		return policyErr
	}
	defer func() {
		if flushErr := flushRecoveryStore(store); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()
	if logicalSectorSize == 0 {
		return fmt.Errorf("run pass-based recovery: logical sector size is required")
	}
	if capacitySectors == 0 {
		return fmt.Errorf("run pass-based recovery: capacity sectors must be greater than zero")
	}
	if source == nil || output == nil || store == nil {
		return fmt.Errorf("run pass-based recovery: source, output, and extent store are required")
	}

	if policy.Fast.Enabled {
		if err := runFastAcquisitionPass(ctx, source, output, logicalSectorSize, capacitySectors, store, policy.Fast.BlockSectors, report); err != nil {
			return err
		}
	}
	if unresolvedSectorCount(store.Extents()) == 0 {
		if err := flushRecoveryStore(store); err != nil {
			return err
		}
		reportRecoveryProgress(report, "Complete", progressExtents(store), nil)
		return nil
	}

	if err := forceRecoveryCheckpoint(store, CheckpointReasonPassTransition); err != nil {
		return err
	}
	if policy.Trim.Enabled {
		if err := runTrimPass(ctx, source, output, logicalSectorSize, store, policy.Trim.AttemptsLimit, report); err != nil {
			return err
		}
	}
	if err := forceRecoveryCheckpoint(store, CheckpointReasonPassTransition); err != nil {
		return err
	}
	for _, adaptive := range policy.Adaptive {
		if !adaptive.Enabled {
			continue
		}
		if err := runAdaptivePass(ctx, source, output, logicalSectorSize, store, uint64(adaptive.BlockSectors), adaptive.AttemptsLimit, fmt.Sprintf("Adaptive recovery (%d-sector reads)", adaptive.BlockSectors), report); err != nil {
			return err
		}
		if err := forceRecoveryCheckpoint(store, CheckpointReasonPassTransition); err != nil {
			return err
		}
	}
	if policy.Targeted.Enabled {
		if err := runAdaptivePass(ctx, source, output, logicalSectorSize, store, uint64(policy.Targeted.BlockSectors), policy.Targeted.AttemptsLimit, "Targeted retry", report); err != nil {
			return err
		}
		if err := forceRecoveryCheckpoint(store, CheckpointReasonPassTransition); err != nil {
			return err
		}
	}
	if policy.FinalizeUnresolved {
		if err := finalizeUnresolvedRanges(store); err != nil {
			return err
		}
	}
	if err := flushRecoveryStore(store); err != nil {
		return err
	}
	finalPass := "Complete"
	if !policy.FinalizeUnresolved && unresolvedSectorCount(store.Extents()) > 0 {
		finalPass = "Deferred work remains"
	}
	reportRecoveryProgress(report, finalPass, progressExtents(store), nil)
	return nil
}

// retryPolicyWithCurrentAttempts gives an explicit user-initiated retry cycle
// its own finite budget while retaining the cumulative attempts written to the
// recovery map. The maximum existing attempt count becomes the cycle offset,
// so no unresolved extent can receive more than one additional policy budget.
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

func runFastAcquisitionPass(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	capacitySectors uint64,
	store recoveryExtentStore,
	blockSectors uint32,
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
		n, readErr := source.ReadAt(buffer[:readSize], offset)
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

func runTrimPass(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	store recoveryExtentStore,
	attemptLimit uint16,
	report func(recoveryPassProgress),
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
			if err := attemptDeferredBlock(ctx, source, output, logicalSectorSize, store, lba, 1, "Trimming deferred ranges", report); err != nil {
				return err
			}
		}
	}
	return nil
}

func runAdaptivePass(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	store recoveryExtentStore,
	blockSectors uint64,
	attemptLimit uint16,
	passName string,
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
			if err := attemptDeferredBlock(ctx, source, output, logicalSectorSize, store, lba, sectorsToRead, passName, report); err != nil {
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
	n, readErr := source.ReadAt(buffer, offset)
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

func fatalRecoveryReadError(readErr error) error {
	if readErr == nil {
		return nil
	}
	if errors.Is(readErr, recovery.ErrStopRequested) {
		return context.Canceled
	}
	if errors.Is(readErr, io.ErrClosedPipe) || errors.Is(readErr, io.ErrUnexpectedEOF) {
		return nil
	}
	if errors.Is(readErr, fs.ErrPermission) || errors.Is(readErr, fs.ErrNotExist) || platformFatalSourceReadError(readErr) {
		return readErr
	}
	return nil
}

func readFailureDetail(action string, lba, sectors uint64, n, expected int, readErr error) string {
	rangeText := fmt.Sprintf("LBA %d-%d", lba, lba+sectors-1)
	if readErr != nil {
		return fmt.Sprintf("%s %s after read error: %v.", action, rangeText, readErr)
	}
	return fmt.Sprintf("%s %s after a short read (%d of %d bytes).", action, rangeText, n, expected)
}

func persistRecoveredRead(output recoverySyncWriter, store recoveryExtentStore, data []byte, offset int64, extent mapfile.Extent) error {
	if persistence, ok := store.(recoveryDataPersistence); ok {
		return persistence.PersistRecovered(data, offset, extent)
	}
	if err := writeFullAtWriter(output, data, offset); err != nil {
		return fmt.Errorf("write recovered data at byte %d: %w", offset, err)
	}
	if err := output.Sync(); err != nil {
		return fmt.Errorf("sync recovered data at byte %d: %w", offset, err)
	}
	if batched, ok := store.(recoveryBatchedStore); ok {
		if err := batched.StageExtent(extent); err != nil {
			return fmt.Errorf("persist recovered extent [%d,%d): %w", extent.StartLBA, extent.EndLBA(), err)
		}
		return nil
	}
	if err := store.ApplyExtent(extent); err != nil {
		return fmt.Errorf("persist recovered extent [%d,%d): %w", extent.StartLBA, extent.EndLBA(), err)
	}
	return nil
}

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

func readSucceeded(n, expected int, err error) bool {
	return n == expected && (err == nil || errors.Is(err, io.EOF))
}

func writeFullAtWriter(writer io.WriterAt, data []byte, offset int64) error {
	written := 0
	for written < len(data) {
		n, err := writer.WriteAt(data[written:], offset+int64(written))
		written += n
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
