//go:build darwin

package platform

import (
	"discrescue/internal/mapfile"
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
		_, recovered, deferred, unreadable := recovery.SummarizeRecoveryExtentStates(extents)
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

func requiredImageBytesDarwin(extents []mapfile.Extent, logicalSectorSize uint32) (uint64, error) {
	var required uint64
	for _, extent := range extents {
		if !recovery.RecoveryStateHasData(extent.State) {
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
