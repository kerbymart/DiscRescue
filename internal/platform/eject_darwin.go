//go:build darwin

package platform

import (
	"errors"
	"fmt"
	"syscall"

	"discrescue/internal/device"
)

func ejectHostOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error) {
	if err := nativeDarwinEject(path, request.Mode == device.EjectForce); err != nil {
		code := device.ErrorDeviceFailure
		if errors.Is(err, syscall.EBUSY) {
			code = device.ErrorBusy
		}
		return device.EjectResult{Requested: request}, &device.OperationError{
			Code: code, Op: "native macOS optical eject",
			Device: device.DeviceRef{Path: path}, Detail: err.Error(), Cause: err,
		}
	}
	return device.EjectResult{
		Requested: request, Status: device.OperationAccepted,
		Verification: device.EjectAcceptedUnverified,
		Detail:       fmt.Sprintf("native macOS eject accepted for %s; drive state will be refreshed", path),
	}, nil
}

func ejectHostCapability(path string) device.Capability {
	return device.Capability{Status: device.SupportSupported, Detail: "macOS diskutil raw-device eject"}
}
