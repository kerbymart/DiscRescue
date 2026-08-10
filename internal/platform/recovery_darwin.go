//go:build darwin

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
)

type OSRecovery struct{}

type darwinRecoveryJob struct {
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

func (j *darwinRecoveryJob) Snapshot() RecoverySnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.refreshTelemetryLocked()
	return j.snapshot
}

func (j *darwinRecoveryJob) refreshTelemetryLocked() {
	if j.telemetry == nil {
		return
	}
	j.snapshot.Telemetry = j.telemetry.Snapshot(j.snapshot.CumulativeRecoveredBytes, j.snapshot.TotalBytes, !j.snapshot.Done)
	j.snapshot.SessionRecoveredBytes = j.snapshot.Telemetry.RecoveredBytes
}

func (j *darwinRecoveryJob) Cancel() { _ = j.RequestStop(recovery.StopIntentPause) }

func (j *darwinRecoveryJob) RequestStop(intent recovery.StopIntent) error {
	j.mu.Lock()
	if j.lifecycle == nil {
		j.mu.Unlock()
		return fmt.Errorf("request stop: lifecycle is unavailable")
	}
	if err := j.lifecycle.RequestStop(intent); err != nil {
		j.mu.Unlock()
		return err
	}
	if j.lifecycle.State() == recovery.JobCancelingRead {
		j.stopTimer = time.AfterFunc(recovery.DefaultStopGracePeriod, func() {
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

func (j *darwinRecoveryJob) ForceStop() error {
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
			return fmt.Errorf("force stop active device request: %w", err)
		}
	}
	cancel()
	return nil
}

func (j *darwinRecoveryJob) setProgress(progress recoveryPassProgress, sectorSize uint32) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.snapshot.CopiedBytes = progress.RecoveredSectors * uint64(sectorSize)
	j.snapshot.CumulativeRecoveredBytes = j.snapshot.CopiedBytes
	j.snapshot.ScannedSectors = progress.ScannedSectors
	j.snapshot.DeferredSectors = progress.DeferredSectors
	j.snapshot.UnreadableSectors = progress.UnreadableSectors
	j.snapshot.Pass = progress.Pass
	j.snapshot.LastIssue = append([]string(nil), progress.LastIssue...)
}

func (j *darwinRecoveryJob) finish(canceled bool, err error) {
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
		} else if canceled {
			_ = j.lifecycle.Checkpointed()
			_ = j.lifecycle.Released(false)
		} else {
			_ = j.lifecycle.Complete()
			_ = j.lifecycle.Checkpointed()
			_ = j.lifecycle.Released(true)
		}
		j.snapshot.State = j.lifecycle.State()
		j.snapshot.CanForceStop = j.lifecycle.CanForceStop()
	}
}

func (OSRecovery) StartImageRecovery(input RecoveryInput) (RecoveryJob, error) {
	if input.DevicePath == "" || input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return nil, fmt.Errorf("start image recovery: device and output paths are required")
	}
	if input.LogicalSectorSize == 0 || input.CapacitySectors == 0 {
		return nil, fmt.Errorf("start image recovery: media geometry is required")
	}
	if _, err := recovery.PolicyForMethod(input.Method); err != nil {
		return nil, fmt.Errorf("start image recovery: %w", err)
	}
	state, resumed, err := openDarwinRecoveryMap(input)
	if err != nil {
		return nil, err
	}
	_, recovered, deferred, unreadable := summarizeRecoveryExtentStates(state.Extents())
	ctx, cancel := context.WithCancel(context.Background())
	job := &darwinRecoveryJob{
		cancel:    cancel,
		state:     state,
		lifecycle: recovery.NewLifecycle(),
		telemetry: recovery.NewTelemetryRecorder(recovery.SystemClock{}, recovered*uint64(input.LogicalSectorSize)),
		snapshot: RecoverySnapshot{
			StartedAt: time.Now(), TotalBytes: input.CapacitySectors * uint64(input.LogicalSectorSize),
			CopiedBytes: recovered * uint64(input.LogicalSectorSize), ScannedSectors: recovered + deferred + unreadable,
			DeferredSectors: deferred, UnreadableSectors: unreadable, Pass: "Fast acquisition", MapPath: state.Path(), Resumed: resumed,
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
	mapPath := darwinRecoveryMapPath(input.OutputPath)
	outputInfo, outputErr := os.Stat(input.OutputPath)
	_, mapErr := os.Stat(mapPath)
	status := RecoveryTargetStatus{OutputPath: input.OutputPath, MapPath: mapPath, RequiredBytes: input.CapacitySectors * uint64(input.LogicalSectorSize)}
	switch {
	case errors.Is(outputErr, os.ErrNotExist) && errors.Is(mapErr, os.ErrNotExist):
		status.CanStartNew = true
		status.Detail = "A new recovery will be created at this path."
		return status, nil
	case outputErr == nil && mapErr == nil:
		_, extents, err := recoverymap.Inspect(mapPath, recoverymap.Geometry{
			LogicalSectorSize:   input.LogicalSectorSize,
			ExpectedSectorCount: input.CapacitySectors,
		})
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: load recovery map %s: %w", mapPath, err)
		}
		requiredBytes, err := requiredImageBytesDarwin(extents, input.LogicalSectorSize)
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: calculate durable image length: %w", err)
		}
		if requiredBytes > uint64(outputInfo.Size()) {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: image %s is smaller than the durable recovery map", input.OutputPath)
		}
		_, recovered, deferred, unreadable := summarizeRecoveryExtentStates(extents)
		status.CanResume = true
		status.RecoveredSectors, status.DeferredSectors, status.UnreadableSectors = recovered, deferred, unreadable
		status.Detail = fmt.Sprintf("Resume recovery from %d recovered sectors, %d deferred sectors, and %d unreadable sectors.", recovered, deferred, unreadable)
		return status, nil
	case outputErr == nil && errors.Is(mapErr, os.ErrNotExist):
		status.Detail = fmt.Sprintf("Output image %s already exists without %s. Choose another output path.", input.OutputPath, mapPath)
		return status, nil
	case errors.Is(outputErr, os.ErrNotExist) && mapErr == nil:
		status.Detail = fmt.Sprintf("Recovery map %s exists without image %s. Choose another output path.", mapPath, input.OutputPath)
		return status, nil
	default:
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: check output %s and map %s", input.OutputPath, mapPath)
	}
}

func (j *darwinRecoveryJob) run(ctx context.Context, input RecoveryInput) {
	rawPath, err := normalizeDarwinOpticalDevice(input.DevicePath)
	if err != nil {
		j.finish(false, fmt.Errorf("open macOS optical source: %w", err))
		return
	}
	sourceFile, err := os.Open(rawPath)
	if err != nil {
		j.finish(false, fmt.Errorf("open macOS optical source %s read-only: %w", rawPath, err))
		return
	}
	source, err := recovery.NewReopenableReaderAt(sourceFile, sourceFile.Close, func() (io.ReaderAt, error) {
		return os.Open(rawPath)
	})
	if err != nil {
		_ = sourceFile.Close()
		j.finish(false, fmt.Errorf("prepare macOS optical source %s: %w", rawPath, err))
		return
	}
	defer source.Close()
	j.mu.Lock()
	j.source = source
	j.mu.Unlock()
	if dir := filepath.Dir(input.OutputPath); dir != "." {
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
	if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
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
	if input.RetryUnresolved {
		policy = retryPolicyWithCurrentAttempts(policy, j.state.Extents())
	}
	err = runPassBasedRecoveryWithPolicy(ctx, lifecycleSource, output, input.LogicalSectorSize, input.CapacitySectors, persistence, policy, func(progress recoveryPassProgress) { j.setProgress(progress, input.LogicalSectorSize) })
	if errors.Is(err, context.Canceled) {
		j.finish(true, nil)
		return
	}
	if err != nil {
		j.finish(false, fmt.Errorf("recover image %s: %w", input.OutputPath, err))
		return
	}
	if err := output.Sync(); err != nil {
		j.finish(false, fmt.Errorf("sync output image %s: %w", input.OutputPath, err))
		return
	}
	j.finish(false, nil)
}

func openDarwinRecoveryMap(input RecoveryInput) (*recoverymap.Store, bool, error) {
	status, err := (OSRecovery{}).InspectRecoveryTarget(input)
	if err != nil {
		return nil, false, err
	}
	if status.CanStartNew {
		if err := preflightDarwinSource(input.DevicePath); err != nil {
			return nil, false, err
		}
		transaction := &recoverymap.StartupTransaction{}
		if err := prepareDarwinOutput(input, transaction); err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		state, resumed, err := createDarwinRecoveryMap(input, status.MapPath)
		if err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		transaction.TrackCreated(status.MapPath)
		transaction.Commit()
		return state, resumed, nil
	}
	if status.CanResume {
		if err := preflightDarwinSource(input.DevicePath); err != nil {
			return nil, false, err
		}
		return loadDarwinRecoveryMap(input, status.MapPath)
	}
	return nil, false, fmt.Errorf("start image recovery: %s", status.Detail)
}

func preflightDarwinSource(devicePath string) error {
	rawPath, err := normalizeDarwinOpticalDevice(devicePath)
	if err != nil {
		return fmt.Errorf("preflight macOS optical source: %w", err)
	}
	source, err := os.Open(rawPath)
	if err != nil {
		return fmt.Errorf("preflight macOS optical source %s read-only: %w", rawPath, err)
	}
	if err := source.Close(); err != nil {
		return fmt.Errorf("preflight macOS optical source %s: close: %w", rawPath, err)
	}
	return nil
}

func prepareDarwinOutput(input RecoveryInput, transaction *recoverymap.StartupTransaction) error {
	if dir := filepath.Dir(input.OutputPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %s: %w", dir, err)
		}
	}
	output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("create output image %s: %w", input.OutputPath, err)
	}
	transaction.TrackCreated(input.OutputPath)
	if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
		_ = output.Close()
		return fmt.Errorf("prepare output image %s: %w", input.OutputPath, err)
	}
	if err := output.Close(); err != nil {
		return fmt.Errorf("close prepared output image %s: %w", input.OutputPath, err)
	}
	return nil
}

func createDarwinRecoveryMap(input RecoveryInput, mapPath string) (*recoverymap.Store, bool, error) {
	header, err := recoveryMapHeader(input)
	if err != nil {
		return nil, false, err
	}
	store, err := recoverymap.Create(mapPath, header)
	return store, false, err
}

func loadDarwinRecoveryMap(input RecoveryInput, mapPath string) (*recoverymap.Store, bool, error) {
	store, err := recoverymap.Open(mapPath, recoverymap.Geometry{
		LogicalSectorSize:   input.LogicalSectorSize,
		ExpectedSectorCount: input.CapacitySectors,
	})
	return store, true, err
}

func darwinRecoveryMapPath(outputPath string) string {
	if strings.HasSuffix(outputPath, ".iso") {
		return strings.TrimSuffix(outputPath, ".iso") + ".drmap"
	}
	return outputPath + ".drmap"
}

func requiredImageBytesDarwin(extents []mapfile.Extent, logicalSectorSize uint32) (uint64, error) {
	var required uint64
	for _, extent := range extents {
		if !recoveryStateHasData(extent.State) {
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
