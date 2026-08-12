//go:build linux

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
	cancel()
	if interruptor, ok := source.(recovery.ReadInterruptor); ok {
		if err := interruptor.Interrupt(); err != nil && !errors.Is(err, io.ErrClosedPipe) {
			return fmt.Errorf("force stop active device request: %w", err)
		}
	}
	return nil
}

func (j *linuxRecoveryJob) report(progress recovery.PassProgress, sectorSize uint32) {
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
