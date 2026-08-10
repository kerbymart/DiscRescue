//go:build linux

package platform

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"discrescue/internal/recovery"
	"discrescue/internal/recoverymap"
)

// OSRecovery is the Linux raw optical recovery adapter. Policy and durable
// state remain shared with the other native adapters.
type OSRecovery struct{}

type linuxRecoveryJob struct {
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

func (j *linuxRecoveryJob) Snapshot() RecoverySnapshot {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.telemetry != nil {
		j.snapshot.Telemetry = j.telemetry.Snapshot(j.snapshot.CumulativeRecoveredBytes, j.snapshot.TotalBytes, !j.snapshot.Done)
		j.snapshot.SessionRecoveredBytes = j.snapshot.Telemetry.RecoveredBytes
	}
	return j.snapshot
}

func (j *linuxRecoveryJob) Cancel() { _ = j.RequestStop(recovery.StopIntentPause) }

func (j *linuxRecoveryJob) RequestStop(intent recovery.StopIntent) error {
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

func (j *linuxRecoveryJob) ForceStop() error {
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
	cancel, source := j.cancel, j.source
	j.mu.Unlock()
	if source != nil {
		if err := source.Close(); err != nil {
			return fmt.Errorf("force stop close source: %w", err)
		}
	}
	cancel()
	return nil
}

func (j *linuxRecoveryJob) report(progress recoveryPassProgress, sectorSize uint32) {
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

func (j *linuxRecoveryJob) finish(canceled bool, err error) {
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
			j.snapshot.ErrText = errors.Join(errors.New(j.snapshot.ErrText), closeErr).Error()
		}
	}
	if j.startup != nil {
		if rollbackErr := j.startup.Rollback(); rollbackErr != nil {
			j.snapshot.ErrText = errors.Join(errors.New(j.snapshot.ErrText), rollbackErr).Error()
		}
	}
	if j.stopTimer != nil {
		j.stopTimer.Stop()
	}
	if j.lifecycle != nil {
		if err != nil {
			j.lifecycle.Fail()
		} else {
			_ = j.lifecycle.Checkpointed()
			_ = j.lifecycle.Released(!canceled)
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
	state, resumed, err := openLinuxRecoveryMap(input)
	if err != nil {
		return nil, err
	}
	_, recovered, deferred, unreadable := summarizeRecoveryExtentStates(state.Extents())
	ctx, cancel := context.WithCancel(context.Background())
	job := &linuxRecoveryJob{
		cancel: cancel, state: state, lifecycle: recovery.NewLifecycle(),
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
	go job.run(ctx, input)
	return job, nil
}

func (OSRecovery) InspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error) {
	if input.OutputPath == "" || input.OutputPath == "Not chosen yet" {
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: output path is not configured")
	}
	mapPath := linuxRecoveryMapPath(input.OutputPath)
	_, outputErr := os.Stat(input.OutputPath)
	_, mapErr := os.Stat(mapPath)
	status := RecoveryTargetStatus{OutputPath: input.OutputPath, MapPath: mapPath, RequiredBytes: input.CapacitySectors * uint64(input.LogicalSectorSize)}
	switch {
	case errors.Is(outputErr, os.ErrNotExist) && errors.Is(mapErr, os.ErrNotExist):
		status.CanStartNew = true
		status.Detail = "A new Linux raw optical recovery will be created at this path."
	case outputErr == nil && mapErr == nil:
		_, extents, err := recoverymap.Inspect(mapPath, recoverymap.Geometry{LogicalSectorSize: input.LogicalSectorSize, ExpectedSectorCount: input.CapacitySectors})
		if err != nil {
			return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: load map %s: %w", mapPath, err)
		}
		_, recovered, deferred, unreadable := summarizeRecoveryExtentStates(extents)
		status.CanResume = true
		status.RecoveredSectors, status.DeferredSectors, status.UnreadableSectors = recovered, deferred, unreadable
		status.Detail = fmt.Sprintf("Resume Linux recovery from %d recovered sectors, %d deferred sectors, and %d unreadable sectors.", recovered, deferred, unreadable)
	case outputErr == nil && errors.Is(mapErr, os.ErrNotExist):
		status.Detail = fmt.Sprintf("Output image %s exists without %s.", input.OutputPath, mapPath)
	case errors.Is(outputErr, os.ErrNotExist) && mapErr == nil:
		status.Detail = fmt.Sprintf("Recovery map %s exists without image %s.", mapPath, input.OutputPath)
	default:
		return RecoveryTargetStatus{}, fmt.Errorf("inspect recovery target: check output %s and map %s", input.OutputPath, mapPath)
	}
	return status, nil
}

func (j *linuxRecoveryJob) run(ctx context.Context, input RecoveryInput) {
	source, err := os.OpenFile(input.DevicePath, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		j.finish(false, fmt.Errorf("open Linux optical source %s read-only: %w", input.DevicePath, err))
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
	policy, err := recovery.PolicyForMethod(input.Method)
	if err != nil {
		j.finish(false, err)
		return
	}
	if input.RetryUnresolved {
		policy = retryPolicyWithCurrentAttempts(policy, j.state.Extents())
	}
	err = runPassBasedRecoveryWithPolicy(ctx, lifecycleSource, output, input.LogicalSectorSize, input.CapacitySectors, persistence, policy, func(progress recoveryPassProgress) { j.report(progress, input.LogicalSectorSize) })
	if errors.Is(err, context.Canceled) {
		j.finish(true, nil)
		return
	}
	if err != nil {
		j.finish(false, fmt.Errorf("recover Linux image %s: %w", input.OutputPath, err))
		return
	}
	if err := output.Sync(); err != nil {
		j.finish(false, fmt.Errorf("sync output image %s: %w", input.OutputPath, err))
		return
	}
	j.finish(false, nil)
}

func openLinuxRecoveryMap(input RecoveryInput) (*recoverymap.Store, bool, error) {
	status, err := (OSRecovery{}).InspectRecoveryTarget(input)
	if err != nil {
		return nil, false, err
	}
	if status.CanStartNew {
		transaction := &recoverymap.StartupTransaction{}
		if dir := filepath.Dir(input.OutputPath); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, false, err
			}
		}
		output, err := os.OpenFile(input.OutputPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o644)
		if err != nil {
			return nil, false, fmt.Errorf("prepare Linux output %s: %w", input.OutputPath, err)
		}
		if err := output.Truncate(int64(input.CapacitySectors) * int64(input.LogicalSectorSize)); err != nil {
			_ = output.Close()
			return nil, false, err
		}
		if err := output.Close(); err != nil {
			return nil, false, err
		}
		transaction.TrackCreated(input.OutputPath)
		header, err := recoveryMapHeader(input)
		if err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		state, err := recoverymap.Create(status.MapPath, header)
		if err != nil {
			_ = transaction.Rollback()
			return nil, false, err
		}
		transaction.TrackCreated(status.MapPath)
		transaction.Commit()
		return state, false, nil
	}
	if status.CanResume {
		state, err := recoverymap.Open(status.MapPath, recoverymap.Geometry{LogicalSectorSize: input.LogicalSectorSize, ExpectedSectorCount: input.CapacitySectors})
		return state, true, err
	}
	return nil, false, fmt.Errorf("start image recovery: %s", status.Detail)
}

func linuxRecoveryMapPath(outputPath string) string {
	if filepath.Ext(outputPath) == ".iso" {
		return outputPath[:len(outputPath)-len(filepath.Ext(outputPath))] + ".drmap"
	}
	return outputPath + ".drmap"
}
