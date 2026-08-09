//go:build darwin

package platform

import (
	"context"
	"strconv"
	"strings"

	"discrescue/internal/device"
)

func ejectHostOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error) {
	if index, ok := parseDrutilDrivePath(path); ok {
		args := []string{"-drive", strconv.Itoa(index), "tray", "eject"}
		if _, err := runDarwinDrutil(context.Background(), args...); err != nil {
			return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorDeviceFailure, Op: "eject macOS optical drive", Device: device.DeviceRef{Path: path}, Detail: strings.TrimSpace(err.Error()), Cause: err}
		}
		return device.EjectResult{Requested: request, Status: device.OperationAccepted, Verification: device.EjectAcceptedUnverified, Detail: "drutil accepted tray eject; drive state will be refreshed"}, nil
	}
	args := []string{"eject"}
	if request.Mode == device.EjectForce {
		args = append(args, "force")
	}
	args = append(args, path)
	if _, err := runDarwinDiskutil(context.Background(), args...); err != nil {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorDeviceFailure, Op: "eject optical drive", Device: device.DeviceRef{Path: path}, Detail: strings.TrimSpace(err.Error()), Cause: err}
	}
	return device.EjectResult{Requested: request, Status: device.OperationAccepted, Verification: device.EjectAcceptedUnverified, Detail: "diskutil accepted eject; media state will be refreshed"}, nil
}

func ejectHostCapability(path string) device.Capability {
	return device.Capability{Status: device.SupportSupported, Detail: "macOS diskutil eject"}
}
