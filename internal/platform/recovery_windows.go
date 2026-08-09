//go:build windows

package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"discrescue/internal/mapfile"
	"discrescue/internal/recovery"
	"discrescue/internal/recoverymap"
	"golang.org/x/sys/windows"
)

type OSRecovery struct{}

type mountedRecoveryJob struct {
	cancel    context.CancelFunc
	state     *recoverymap.Store
	startup   *recoverymap.StartupTransaction
	lifecycle *recovery.Lifecycle
	stopTimer *time.Timer
	source    io.Closer
	telemetry *recovery.TelemetryRecorder

	mu       sync.Mutex
	snapshot RecoverySnapshot
}

func (j *mountedRecoveryJob) Snapshot() RecoverySnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.refreshTelemetryLocked()
	return j.snapshot
}

func (j *mountedRecoveryJob) refreshTelemetryLocked() {
	if j.telemetry == nil {
		return
	}
	j.snapshot.Telemetry = j.telemetry.Snapshot(j.snapshot.CumulativeRecoveredBytes, j.snapshot.TotalBytes, !j.snapshot.Done)
	j.snapshot.SessionRecoveredBytes = j.snapshot.Telemetry.RecoveredBytes
}

func (j *mountedRecoveryJob) Cancel() {
	_ = j.RequestStop(recovery.StopIntentPause)
}

func (j *mountedRecoveryJob) RequestStop(intent recovery.StopIntent) error {
	j.mu.Lock()
	if j.lifecycle == nil {
		j.mu.Unlock()
		return fmt.Errorf("request stop: lifecycle is unavailable")
	}
	err := j.lifecycle.RequestStop(intent)
	if err != nil {
		j.mu.Unlock()
		return err
	}
	if j.lifecycle.State() == recovery.JobCancelingRead {
		j.stopTimer = time.AfterFunc(5*time.Second, func() {
			j.mu.Lock()
			_ = j.lifecycle.GraceExpired()
			j.snapshot.State = j.lifecycle.State()
			j.snapshot.CanForceStop = j.lifecycle.CanForceStop()
			j.mu.Unlock()
		})
	}
	j.snapshot.State = j.lifecycle.State()
	j.snapshot.StopIntent = intent
	j.snapshot.CanForceStop = j.lifecycle.CanForceStop()
	cancel := j.cancel
	j.mu.Unlock()
	cancel()
	return nil
}

func (j *mountedRecoveryJob) ForceStop() error {
	j.mu.Lock()
	if j.lifecycle == nil {
		j.mu.Unlock()
		return fmt.Errorf("force stop: lifecycle is unavailable")
	}
	if err := j.lifecycle.ForceStop(); err != nil {
		j.mu.Unlock()
		return err
	}
	j.snapshot.State = j.lifecycle.State()
	j.snapshot.CanForceStop = false
	cancel := j.cancel
	source := j.source
	j.mu.Unlock()
	if source != nil {
		if err := source.Close(); err != nil {
			return fmt.Errorf("force stop close source: %w", err)
		}
	}
	cancel()
	return nil
}

func (j *mountedRecoveryJob) setPassProgress(progress recoveryPassProgress, logicalSectorSize uint32) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.snapshot.CopiedBytes = progress.RecoveredSectors * uint64(logicalSectorSize)
	j.snapshot.CumulativeRecoveredBytes = j.snapshot.CopiedBytes
	j.snapshot.ScannedSectors = progress.ScannedSectors
	j.snapshot.DeferredSectors = progress.DeferredSectors
	j.snapshot.UnreadableSectors = progress.UnreadableSectors
	j.snapshot.Pass = progress.Pass
	lines := []string{
		fmt.Sprintf(
			"Scanned %d sectors; recovered %d; deferred %d; unreadable %d.",
			progress.ScannedSectors,
			progress.RecoveredSectors,
			progress.DeferredSectors,
			progress.UnreadableSectors,
		),
	}
	lines = append(lines, progress.LastIssue...)
	j.snapshot.LastIssue = lines
}

func (j *mountedRecoveryJob) finish(canceled bool, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.snapshot.Done = true
	j.snapshot.Canceled = canceled
	j.snapshot.EndedAt = time.Now()
	if err != nil {
		j.snapshot.ErrText = err.Error()
	}
	if j.state != nil {
		if closeErr := j.state.Close(canceled || err == nil); closeErr != nil {
			if j.snapshot.ErrText == "" {
				j.snapshot.ErrText = closeErr.Error()
			} else {
				j.snapshot.ErrText += "; finalize recovery map: " + closeErr.Error()
			}
		}
	}
	if j.startup != nil {
		if rollbackErr := j.startup.Rollback(); rollbackErr != nil {
			if j.snapshot.ErrText == "" {
				j.snapshot.ErrText = rollbackErr.Error()
			} else {
				j.snapshot.ErrText += "; rollback startup artifacts: " + rollbackErr.Error()
			}
		}
	}
	if j.stopTimer != nil {
		j.stopTimer.Stop()
	}
	if j.lifecycle != nil {
		if err != nil {
			j.lifecycle.Fail()
		} else {
			if canceled {
				_ = j.lifecycle.Checkpointed()
				_ = j.lifecycle.Released(false)
			} else {
				_ = j.lifecycle.Complete()
				_ = j.lifecycle.Checkpointed()
				_ = j.lifecycle.Released(true)
			}
		}
		j.snapshot.State = j.lifecycle.State()
		j.snapshot.CanForceStop = j.lifecycle.CanForceStop()
	}
}

func (OSRecovery) StartImageRecovery(input RecoveryInput) (RecoveryJob, error) {
	if input.DevicePath == "" {
		return nil, fmt.Errorf("start image recovery: device path is required")
	}
	if input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return nil, fmt.Errorf("start image recovery: output path is not configured")
	}
	if input.LogicalSectorSize == 0 {
		return nil, fmt.Errorf("start image recovery: logical sector size is required")
	}
	if input.CapacitySectors == 0 {
		return nil, fmt.Errorf("start image recovery: capacity sectors must be greater than zero")
	}
	if _, err := recovery.PolicyForMethod(input.Method); err != nil {
		return nil, fmt.Errorf("start image recovery: %w", err)
	}

	state, resumed, err := openRecoveryMapState(input)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	scannedSectors, recoveredSectors, deferredSectors, unreadableSectors := summarizeRecoveryExtentStates(state.Extents())
	lastIssue := []string{}
	if resumed {
		lastIssue = []string{
			fmt.Sprintf(
				"Resuming durable state: scanned %d, recovered %d, deferred %d, unreadable %d sectors.",
				scannedSectors,
				recoveredSectors,
				deferredSectors,
				unreadableSectors,
			),
		}
	}
	job := &mountedRecoveryJob{
		cancel:    cancel,
		state:     state,
		lifecycle: recovery.NewLifecycle(),
		telemetry: recovery.NewTelemetryRecorder(recovery.SystemClock{}, recoveredSectors*uint64(input.LogicalSectorSize)),
		snapshot: RecoverySnapshot{
			StartedAt:         time.Now(),
			TotalBytes:        input.CapacitySectors * uint64(input.LogicalSectorSize),
			CopiedBytes:       recoveredSectors * uint64(input.LogicalSectorSize),
			ScannedSectors:    scannedSectors,
			DeferredSectors:   deferredSectors,
			UnreadableSectors: unreadableSectors,
			Pass:              "Fast acquisition",
			MapPath:           state.Path(),
			Resumed:           resumed,
			LastIssue:         lastIssue,
		},
	}
	if err := job.lifecycle.Start(); err != nil {
		cancel()
		_ = state.Close(false)
		return nil, err
	}
	job.snapshot.State = job.lifecycle.State()
	job.snapshot.Method = input.Method
	job.refreshTelemetryLocked()
	go job.run(ctx, input)
	return job, nil
}

func (OSRecovery) InspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error) {
	if input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: output path is not configured")
	}
	status, err := inspectRecoveryTarget(input)
	if err != nil {
		return RecoveryTargetStatus{}, err
	}
	return status, nil
}

func (j *mountedRecoveryJob) run(ctx context.Context, input RecoveryInput) {
	rawPath := rawVolumePath(input.DevicePath)
	source, err := os.Open(rawPath)
	if err != nil {
		j.finish(false, fmt.Errorf("open source volume %s: %w", rawPath, err))
		return
	}
	defer source.Close()
	j.mu.Lock()
	j.source = source
	j.mu.Unlock()

	if dir := filepath.Dir(input.OutputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			j.finish(false, fmt.Errorf("create output directory %s: %w", dir, err))
			return
		}
	}
	flags := os.O_RDWR
	if j.startup != nil {
		flags |= os.O_CREATE | os.O_EXCL
	} else {
		flags |= os.O_CREATE
	}
	output, err := os.OpenFile(input.OutputPath, flags, 0o644)
	if err != nil {
		j.finish(false, fmt.Errorf("create output image %s: %w", input.OutputPath, err))
		return
	}
	if j.startup != nil {
		j.startup.TrackCreated(input.OutputPath)
	}
	defer output.Close()

	totalBytes := input.CapacitySectors * uint64(input.LogicalSectorSize)
	if err := output.Truncate(int64(totalBytes)); err != nil {
		j.finish(false, fmt.Errorf("prepare output image %s: %w", input.OutputPath, err))
		return
	}
	if j.startup != nil {
		j.startup.Commit()
	}

	persistence := newRecoveryPersistence(output, j.state)
	lifecycleSource := &recovery.LifecycleReaderAt{Source: source, Lifecycle: j.lifecycle}
	policy, policyErr := recovery.PolicyForMethod(input.Method)
	if policyErr != nil {
		j.finish(false, policyErr)
		return
	}
	err = runPassBasedRecoveryWithPolicy(
		ctx,
		lifecycleSource,
		output,
		input.LogicalSectorSize,
		input.CapacitySectors,
		persistence,
		policy,
		func(progress recoveryPassProgress) {
			j.setPassProgress(progress, input.LogicalSectorSize)
		},
	)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			j.finish(true, nil)
			return
		}
		j.finish(false, fmt.Errorf("recover image %s: %w", input.OutputPath, err))
		return
	}

	if err := output.Sync(); err != nil {
		j.finish(false, fmt.Errorf("sync output image %s: %w", input.OutputPath, err))
		return
	}
	j.finish(false, nil)
}

func openRecoveryMapState(input RecoveryInput) (*recoverymap.Store, bool, error) {
	status, err := inspectRecoveryTarget(input)
	if err != nil {
		return nil, false, err
	}
	switch {
	case status.CanStartNew:
		mapPath := recoveryMapPath(input.OutputPath)
		if err := preflightWindowsSource(input.DevicePath); err != nil {
			return nil, false, err
		}
		transaction := &recoverymap.StartupTransaction{}
		if err := prepareWindowsOutput(input, transaction); err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		state, resumed, err := createRecoveryMapState(input, mapPath)
		if err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		transaction.TrackCreated(mapPath)
		transaction.Commit()
		return state, resumed, nil
	case status.CanResume:
		if err := preflightWindowsSource(input.DevicePath); err != nil {
			return nil, false, err
		}
		return loadRecoveryMapState(input, status.MapPath)
	default:
		return nil, false, fmt.Errorf("start image recovery: %s", status.Detail)
	}
}

func preflightWindowsSource(devicePath string) error {
	rawPath := rawVolumePath(devicePath)
	source, err := os.Open(rawPath)
	if err != nil {
		return fmt.Errorf("preflight source volume %s: %w", rawPath, err)
	}
	if err := source.Close(); err != nil {
		return fmt.Errorf("preflight source volume %s: close: %w", rawPath, err)
	}
	return nil
}

func prepareWindowsOutput(input RecoveryInput, transaction *recoverymap.StartupTransaction) error {
	if dir := filepath.Dir(input.OutputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %s: %w", dir, err)
		}
	}
	output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("create output image %s: %w", input.OutputPath, err)
	}
	transaction.TrackCreated(input.OutputPath)
	totalBytes := input.CapacitySectors * uint64(input.LogicalSectorSize)
	if err := output.Truncate(int64(totalBytes)); err != nil {
		_ = output.Close()
		return fmt.Errorf("prepare output image %s: %w", input.OutputPath, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close prepared output image %s: %w", input.OutputPath, err)
	}
	return nil
}

func createRecoveryMapState(input RecoveryInput, mapPath string) (*recoverymap.Store, bool, error) {
	store, err := recoverymap.Create(mapPath, mapfile.Header{
		LogicalSectorSize:   input.LogicalSectorSize,
		ExpectedSectorCount: input.CapacitySectors,
		OutputFormat:        1,
		CreationUnixNano:    time.Now().UnixNano(),
	})
	return store, false, err
}

func loadRecoveryMapState(input RecoveryInput, mapPath string) (*recoverymap.Store, bool, error) {
	store, err := recoverymap.Open(mapPath, recoverymap.Geometry{
		LogicalSectorSize:   input.LogicalSectorSize,
		ExpectedSectorCount: input.CapacitySectors,
	})
	return store, true, err
}

func inspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error) {
	mapPath := recoveryMapPath(input.OutputPath)
	requiredBytes := input.CapacitySectors * uint64(input.LogicalSectorSize)
	availableBytes, spaceKnown, spaceErr := availableBytesForOutputPath(input.OutputPath)
	outputInfo, outputErr := os.Stat(input.OutputPath)
	_, mapErr := os.Stat(mapPath)

	switch {
	case errors.Is(outputErr, os.ErrNotExist) && errors.Is(mapErr, os.ErrNotExist):
		if spaceErr == nil && spaceKnown && availableBytes < requiredBytes {
			return RecoveryTargetStatus{
				OutputPath:     input.OutputPath,
				MapPath:        mapPath,
				RequiredBytes:  requiredBytes,
				AvailableBytes: availableBytes,
				SpaceKnown:     true,
				Detail: fmt.Sprintf(
					"The selected output drive does not have enough free space for this image. Need %s and only %s are free. Choose another output path.",
					formatBytes(requiredBytes),
					formatBytes(availableBytes),
				),
			}, nil
		}
		return RecoveryTargetStatus{
			OutputPath:     input.OutputPath,
			MapPath:        mapPath,
			CanStartNew:    true,
			RequiredBytes:  requiredBytes,
			AvailableBytes: availableBytes,
			SpaceKnown:     spaceKnown && spaceErr == nil,
			Detail:         "A new recovery will be created at this path.",
		}, nil
	case outputErr == nil && mapErr == nil:
		_, extents, err := recoverymap.Inspect(mapPath, recoverymap.Geometry{
			LogicalSectorSize:   input.LogicalSectorSize,
			ExpectedSectorCount: input.CapacitySectors,
		})
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: load recovery map %s: %w", mapPath, err)
		}
		requiredBytes, err := requiredImageBytes(extents, input.LogicalSectorSize)
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: calculate durable image length: %w", err)
		}
		if requiredBytes > uint64(outputInfo.Size()) {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: image %s is smaller than the durable recovery map", input.OutputPath)
		}
		_, recoveredSectors, deferredSectors, unreadableSectors := summarizeRecoveryExtentStates(extents)
		return RecoveryTargetStatus{
			OutputPath:        input.OutputPath,
			MapPath:           mapPath,
			CanResume:         true,
			RecoveredSectors:  recoveredSectors,
			DeferredSectors:   deferredSectors,
			UnreadableSectors: unreadableSectors,
			RequiredBytes:     requiredBytes,
			AvailableBytes:    availableBytes,
			SpaceKnown:        spaceKnown && spaceErr == nil,
			Detail: fmt.Sprintf(
				"Resume recovery from %s recovered sectors, %s deferred sectors, and %s unreadable sectors.",
				formatUint(recoveredSectors),
				formatUint(deferredSectors),
				formatUint(unreadableSectors),
			),
		}, nil
	case outputErr == nil && errors.Is(mapErr, os.ErrNotExist):
		return RecoveryTargetStatus{
			OutputPath:     input.OutputPath,
			MapPath:        mapPath,
			RequiredBytes:  requiredBytes,
			AvailableBytes: availableBytes,
			SpaceKnown:     spaceKnown && spaceErr == nil,
			Detail:         fmt.Sprintf("Output image %s already exists without %s. Choose another output path.", input.OutputPath, mapPath),
		}, nil
	case errors.Is(outputErr, os.ErrNotExist) && mapErr == nil:
		return RecoveryTargetStatus{
			OutputPath:     input.OutputPath,
			MapPath:        mapPath,
			RequiredBytes:  requiredBytes,
			AvailableBytes: availableBytes,
			SpaceKnown:     spaceKnown && spaceErr == nil,
			Detail:         fmt.Sprintf("Recovery map %s exists without image %s. Choose another output path.", mapPath, input.OutputPath),
		}, nil
	case outputErr != nil && !errors.Is(outputErr, os.ErrNotExist):
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: check output image %s: %w", input.OutputPath, outputErr)
	default:
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: check recovery map %s: %w", mapPath, mapErr)
	}
}

func availableBytesForOutputPath(outputPath string) (uint64, bool, error) {
	absolutePath, err := filepath.Abs(outputPath)
	if err != nil {
		return 0, false, fmt.Errorf("resolve output path %s: %w", outputPath, err)
	}
	volume := filepath.VolumeName(absolutePath)
	if volume == "" {
		return 0, false, nil
	}
	root := volume + `\`
	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return 0, false, fmt.Errorf("encode output root %s: %w", root, err)
	}
	var freeCaller uint64
	var totalBytes uint64
	var freeTotal uint64
	if err := windows.GetDiskFreeSpaceEx(rootPtr, &freeCaller, &totalBytes, &freeTotal); err != nil {
		return 0, false, fmt.Errorf("check free space for %s: %w", root, err)
	}
	return freeTotal, true, nil
}

func summarizeExtents(extents []mapfile.Extent, logicalSectorSize uint32) (uint64, uint64) {
	_, recoveredSectors, _, unreadable := summarizeRecoveryExtentStates(extents)
	return recoveredSectors * uint64(logicalSectorSize), unreadable
}

func summarizeExtentsToSectors(extents []mapfile.Extent) (uint64, uint64) {
	_, recoveredSectors, _, unreadable := summarizeRecoveryExtentStates(extents)
	return recoveredSectors, unreadable
}

func claimsImageData(state mapfile.SectorState) bool {
	return recoveryStateHasData(state)
}

func requiredImageBytes(extents []mapfile.Extent, logicalSectorSize uint32) (uint64, error) {
	var required uint64
	for _, extent := range extents {
		if !claimsImageData(extent.State) {
			continue
		}
		endLBA, err := extent.CheckedEndLBA()
		if err != nil {
			return 0, err
		}
		end, err := mapfile.CheckedSectorByteOffset(endLBA, logicalSectorSize)
		if err != nil {
			return 0, err
		}
		if end > required {
			required = end
		}
	}
	return required, nil
}

func mustMarshalHeader(header mapfile.Header) []byte {
	data, err := mapfile.MarshalHeader(header)
	if err != nil {
		panic(err)
	}
	return data
}

func recoveryMapPath(outputPath string) string {
	if strings.HasSuffix(outputPath, ".iso") {
		return strings.TrimSuffix(outputPath, ".iso") + ".drmap"
	}
	return outputPath + ".drmap"
}

func formatUint(value uint64) string {
	return fmt.Sprintf("%d", value)
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(div), "KMGTPE"[exp])
}
