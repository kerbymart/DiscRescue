package recovery

import (
	"context"
	"errors"
	"fmt"
	"io"

	"discrescue/internal/mapfile"
)

const (
	fastPassSectors     = uint64(64)
	maxTargetedAttempts = uint16(6)
)

type ExtentStore interface {
	Extents() []mapfile.Extent
	ApplyExtent(mapfile.Extent) error
}

type BatchedExtentStore interface {
	StageExtent(mapfile.Extent) error
	Flush() error
	PendingBytes() uint64
}

type DurableExtentSnapshot interface {
	DurableExtents() []mapfile.Extent
}

type DataPersistence interface {
	ExtentStore
	PersistRecovered(data []byte, offset int64, extent mapfile.Extent) error
	ForceCheckpoint(CheckpointReason) error
}

type SyncWriterAt interface {
	io.WriterAt
	Sync() error
}

type PassProgress struct {
	Pass              string
	ScannedSectors    uint64
	RecoveredSectors  uint64
	DeferredSectors   uint64
	UnreadableSectors uint64
	LastIssue         []string
}

type ReadErrorClassifier func(error) error

type recoveryExtentStore = ExtentStore
type recoveryBatchedStore = BatchedExtentStore
type recoveryDurableSnapshot = DurableExtentSnapshot
type recoveryDataPersistence = DataPersistence
type recoverySyncWriter = SyncWriterAt
type recoveryPassProgress = PassProgress

func runPassBasedRecovery(
	ctx context.Context,
	source io.ReaderAt,
	output recoverySyncWriter,
	logicalSectorSize uint32,
	capacitySectors uint64,
	store recoveryExtentStore,
	report func(recoveryPassProgress),
) (err error) {
	policy, policyErr := PolicyForMethod(RecoveryMethodBalanced)
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
	policy RecoveryPolicy,
	report func(recoveryPassProgress),
) (err error) {
	return RunPassBasedRecoveryWithPolicy(ctx, source, output, logicalSectorSize, capacitySectors, store, policy, report, nil)
}

// RunPassBasedRecoveryWithPolicy executes the platform-neutral recovery
// policy against caller-owned source, output, and extent-store capabilities.
// The optional classifier lets a platform preserve fatal native-source errors
// without importing platform code into this package.
func RunPassBasedRecoveryWithPolicy(
	ctx context.Context,
	source io.ReaderAt,
	output SyncWriterAt,
	logicalSectorSize uint32,
	capacitySectors uint64,
	store ExtentStore,
	policy RecoveryPolicy,
	report func(PassProgress),
	classify ReadErrorClassifier,
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
		if err := runFastAcquisitionPass(ctx, source, output, logicalSectorSize, capacitySectors, store, policy.Fast.BlockSectors, policy.ReadDeadlines, report, classify); err != nil {
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
		if err := runTrimPass(ctx, source, output, logicalSectorSize, store, policy.Trim.AttemptsLimit, policy.ReadDeadlines, report, classify); err != nil {
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
		if err := runAdaptivePass(ctx, source, output, logicalSectorSize, store, uint64(adaptive.BlockSectors), adaptive.AttemptsLimit, fmt.Sprintf("Adaptive recovery (%d-sector reads)", adaptive.BlockSectors), policy.ReadDeadlines, report, classify); err != nil {
			return err
		}
		if err := forceRecoveryCheckpoint(store, CheckpointReasonPassTransition); err != nil {
			return err
		}
	}
	if policy.Targeted.Enabled {
		if err := runAdaptivePass(ctx, source, output, logicalSectorSize, store, uint64(policy.Targeted.BlockSectors), policy.Targeted.AttemptsLimit, "Targeted retry", policy.ReadDeadlines, report, classify); err != nil {
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
