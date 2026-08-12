//go:build darwin

package platform

import (
	"context"
	"discrescue/internal/recovery"
	"discrescue/internal/recoverymap"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

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
	cancel()
	if interruptor, ok := source.(recovery.ReadInterruptor); ok {
		if err := interruptor.Interrupt(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			return fmt.Errorf("force stop active device request: %w", err)
		}
	}
	return nil
}

func (j *darwinRecoveryJob) setProgress(progress recovery.PassProgress, sectorSize uint32) {
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
