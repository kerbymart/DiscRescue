//go:build !linux && !darwin && !windows

package platform

import (
	"discrescue/internal/device"
	"fmt"
)

func ejectHostOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error) {
	return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorUnsupported, Op: "eject optical drive", Device: device.DeviceRef{Path: path}, Detail: fmt.Sprintf("%s eject is not implemented", request.Mode)}
}

func ejectHostCapability(path string) device.Capability {
	return device.Capability{Status: device.SupportUnsupported, Detail: fmt.Sprintf("%s eject is not implemented", path)}
}
