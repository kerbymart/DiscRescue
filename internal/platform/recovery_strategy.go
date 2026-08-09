package platform

import (
	"context"
	"errors"
	"fmt"
	"io"

	"discrescue/internal/mapfile"
	"discrescue/internal/recovery"
)

const (
	fastPassSectors             = uint64(64)
	maxFastConsecutiveFailures  = 8
	maxRetryConsecutiveFailures = 128
	maxTargetedAttempts         = uint16(6)
)

var errRecoveryConsecutiveFailures = errors.New("recovery stopped after too many consecutive read failures")

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

type retryBudget struct {
	consecutiveFailures    int
	maxConsecutiveFailures int
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
		if err := runFastAcquisitionPass(ctx, source, output, logicalSectorSize, capacitySectors, store, policy.Fast.BlockSectors, policy.Fast.MaxConsecutiveFailures, report); err != nil {
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
	budget := &retryBudget{maxConsecutiveFailures: int(policy.RetryMaxConsecutiveFailures)}
	if policy.Trim.Enabled {
		if err := runTrimPass(ctx, source, output, logicalSectorSize, store, budget, policy.Trim.AttemptsLimit, report); err != nil {
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
		if err := runAdaptivePass(ctx, source, output, logicalSectorSize, store, uint64(adaptive.BlockSectors), adaptive.AttemptsLimit, fmt.Sprintf("Adaptive recovery (%d-sector reads)", adaptive.BlockSectors), budget, report); err != nil {
			return err
		}
		if err := forceRecoveryCheckpoint(store, CheckpointReasonPassTransition); err != nil {
			return err
		}
	}
	if policy.Targeted.Enabled {
		if err := runAdaptivePass(ctx, source, output, logicalSectorSize, store, uint64(policy.Targeted.BlockSectors), policy.Targeted.AttemptsLimit, "Targeted retry", budget, report); err != nil {
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

func runFastAcquisitionPass(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	capacitySectors uint64,
	store recoveryExtentStore,
	blockSectors uint32,
	maxFailures uint32,
	report func(recoveryPassProgress),
) error {
	if blockSectors == 0 || maxFailures == 0 {
		return fmt.Errorf("run fast acquisition pass: invalid policy limits")
	}
	sectorSize := uint64(logicalSectorSize)
	buffer := make([]byte, uint64(blockSectors)*sectorSize)
	consecutiveFailures := 0

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
			consecutiveFailures = 0
			reportRecoveryProgress(report, "Fast acquisition", progressExtents(store), nil)
			lba += sectorsToRead
			continue
		}

		deferred := mapfile.Extent{
			StartLBA:   lba,
			Sectors:    uint32(sectorsToRead),
			State:      mapfile.SectorStateUnknown,
			Confidence: mapfile.ConfidenceNone,
			Attempts:   1,
		}
		if err := store.ApplyExtent(deferred); err != nil {
			return fmt.Errorf("defer failed range [%d,%d): %w", lba, lba+sectorsToRead, err)
		}
		consecutiveFailures++
		reportRecoveryProgress(report, "Fast acquisition", progressExtents(store), []string{
			fmt.Sprintf("Deferred LBA %d-%d after the first block read failed.", lba, lba+sectorsToRead-1),
			"Continuing forward; smaller bounded reads will revisit this range later.",
		})
		lba += sectorsToRead
		if uint32(consecutiveFailures) >= maxFailures {
			return fmt.Errorf("%w during fast acquisition (%d failed blocks)", errRecoveryConsecutiveFailures, consecutiveFailures)
		}
	}
	return nil
}

func runTrimPass(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	store recoveryExtentStore,
	budget *retryBudget,
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
			if err := attemptDeferredBlock(ctx, source, output, logicalSectorSize, store, lba, 1, budget, "Trimming deferred ranges", report); err != nil {
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
	budget *retryBudget,
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
			if err := attemptDeferredBlock(ctx, source, output, logicalSectorSize, store, lba, sectorsToRead, budget, passName, report); err != nil {
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
	budget *retryBudget,
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
		budget.consecutiveFailures = 0
		reportRecoveryProgress(report, passName, progressExtents(store), nil)
		return nil
	}

	deferred := queued
	deferred.State = mapfile.SectorStateUnknown
	if err := store.ApplyExtent(deferred); err != nil {
		return fmt.Errorf("restore deferred range [%d,%d): %w", lba, lba+sectorsToRead, err)
	}
	budget.consecutiveFailures++
	reportRecoveryProgress(report, passName, progressExtents(store), []string{
		fmt.Sprintf("Retry %d failed for LBA %d-%d.", attempts, lba, lba+sectorsToRead-1),
		"The range remains deferred until its bounded retry budget is exhausted.",
	})
	if budget.maxConsecutiveFailures > 0 && uint32(budget.consecutiveFailures) >= uint32(budget.maxConsecutiveFailures) {
		return fmt.Errorf("%w during error recovery (%d failed reads)", errRecoveryConsecutiveFailures, budget.consecutiveFailures)
	}
	return nil
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
