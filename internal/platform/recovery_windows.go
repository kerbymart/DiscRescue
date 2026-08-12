//go:build windows

package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"discrescue/internal/mapfile"
	"discrescue/internal/recovery"
)

type OSRecovery struct{}

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
	scannedSectors, recoveredSectors, deferredSectors, unreadableSectors := recovery.SummarizeRecoveryExtentStates(state.Extents())
	lastIssue := []string{}
	if resumed {
		lastIssue = []string{fmt.Sprintf("Resuming durable state: scanned %d, recovered %d, deferred %d, unreadable %d sectors.", scannedSectors, recoveredSectors, deferredSectors, unreadableSectors)}
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

func summarizeExtents(extents []mapfile.Extent, logicalSectorSize uint32) (uint64, uint64) {
	_, recoveredSectors, _, unreadable := recovery.SummarizeRecoveryExtentStates(extents)
	return recoveredSectors * uint64(logicalSectorSize), unreadable
}

func summarizeExtentsToSectors(extents []mapfile.Extent) (uint64, uint64) {
	_, recoveredSectors, _, unreadable := recovery.SummarizeRecoveryExtentStates(extents)
	return recoveredSectors, unreadable
}

func claimsImageData(state mapfile.SectorState) bool {
	return recovery.RecoveryStateHasData(state)
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
