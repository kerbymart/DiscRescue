//go:build darwin

package platform

import (
	"context"
	"fmt"
	"time"

	"discrescue/internal/recovery"
)

type OSRecovery struct{}

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
