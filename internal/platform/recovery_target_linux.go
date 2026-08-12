//go:build linux

package platform

import (
	"discrescue/internal/recovery"
	"discrescue/internal/recoverymap"
	"errors"
	"fmt"
	"os"
)

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
		_, recovered, deferred, unreadable := recovery.SummarizeRecoveryExtentStates(extents)
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
