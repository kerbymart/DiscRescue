//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"discrescue/internal/mapfile"
	"discrescue/internal/recoverymap"
	"golang.org/x/sys/windows"
)

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
				OutputPath: input.OutputPath, MapPath: mapPath, RequiredBytes: requiredBytes,
				AvailableBytes: availableBytes, SpaceKnown: true,
				Detail: fmt.Sprintf("The selected output drive does not have enough free space for this image. Need %s and only %s are free. Choose another output path.", formatBytes(requiredBytes), formatBytes(availableBytes)),
			}, nil
		}
		return RecoveryTargetStatus{OutputPath: input.OutputPath, MapPath: mapPath, CanStartNew: true, RequiredBytes: requiredBytes, AvailableBytes: availableBytes, SpaceKnown: spaceKnown && spaceErr == nil, Detail: "A new recovery will be created at this path."}, nil
	case outputErr == nil && mapErr == nil:
		_, extents, err := recoverymap.Inspect(mapPath, recoverymap.Geometry{LogicalSectorSize: input.LogicalSectorSize, ExpectedSectorCount: input.CapacitySectors})
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
		return RecoveryTargetStatus{OutputPath: input.OutputPath, MapPath: mapPath, CanResume: true, RecoveredSectors: recoveredSectors, DeferredSectors: deferredSectors, UnreadableSectors: unreadableSectors, RequiredBytes: requiredBytes, AvailableBytes: availableBytes, SpaceKnown: spaceKnown && spaceErr == nil, Detail: fmt.Sprintf("Resume recovery from %s recovered sectors, %s deferred sectors, and %s unreadable sectors.", formatUint(recoveredSectors), formatUint(deferredSectors), formatUint(unreadableSectors))}, nil
	case outputErr == nil && errors.Is(mapErr, os.ErrNotExist):
		return RecoveryTargetStatus{OutputPath: input.OutputPath, MapPath: mapPath, RequiredBytes: requiredBytes, AvailableBytes: availableBytes, SpaceKnown: spaceKnown && spaceErr == nil, Detail: fmt.Sprintf("Output image %s already exists without %s. Choose another output path.", input.OutputPath, mapPath)}, nil
	case errors.Is(outputErr, os.ErrNotExist) && mapErr == nil:
		return RecoveryTargetStatus{OutputPath: input.OutputPath, MapPath: mapPath, RequiredBytes: requiredBytes, AvailableBytes: availableBytes, SpaceKnown: spaceKnown && spaceErr == nil, Detail: fmt.Sprintf("Recovery map %s exists without image %s. Choose another output path.", mapPath, input.OutputPath)}, nil
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
