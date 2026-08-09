//go:build windows

package platform

import (
	"fmt"

	"discrescue/internal/device"
	"golang.org/x/sys/windows"
)

const ioctlStorageEjectMedia = 0x002D4808

func ejectHostOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error) {
	handle, err := windows.CreateFile(windows.StringToUTF16Ptr(path), windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorPermissionDenied, Op: "open drive for eject", Device: device.DeviceRef{Path: path}, Cause: err}
	}
	defer windows.CloseHandle(handle)
	var returned uint32
	if err := windows.DeviceIoControl(handle, ioctlStorageEjectMedia, nil, 0, nil, 0, &returned, nil); err != nil {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorDeviceFailure, Op: "eject optical drive", Device: device.DeviceRef{Path: path}, Detail: fmt.Sprintf("native ioctl: %v", err), Cause: err}
	}
	return device.EjectResult{Requested: request, Status: device.OperationAccepted, Verification: device.EjectAcceptedUnverified, Detail: "Windows eject request accepted; media state will be refreshed"}, nil
}

func ejectHostCapability(path string) device.Capability {
	return device.Capability{Status: device.SupportSupported, Detail: "Windows storage eject ioctl"}
}
