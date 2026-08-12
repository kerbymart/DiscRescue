//go:build windows

package platform

import (
	"errors"
	"fmt"

	"discrescue/internal/device"
	"golang.org/x/sys/windows"
)

const ioctlStorageEjectMedia = 0x002D4808

func ejectHostOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error) {
	if request.Mode == device.EjectForce {
		return device.EjectResult{Requested: request}, &device.OperationError{
			Code:   device.ErrorUnsupported,
			Op:     "force eject optical drive",
			Device: device.DeviceRef{Path: path},
			Detail: "Windows provides no distinct force-eject operation",
		}
	}
	volumePath := rawVolumePath(path)
	handle, err := windows.CreateFile(windows.StringToUTF16Ptr(volumePath), windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return device.EjectResult{Requested: request}, windowsEjectOperationError("open drive for eject", path, err)
	}
	defer windows.CloseHandle(handle)
	var returned uint32
	if err := windows.DeviceIoControl(handle, ioctlStorageEjectMedia, nil, 0, nil, 0, &returned, nil); err != nil {
		return device.EjectResult{Requested: request}, windowsEjectOperationError("eject optical drive", path, err)
	}
	return device.EjectResult{Requested: request, Status: device.OperationAccepted, Verification: device.EjectAcceptedUnverified, Detail: "Windows eject request accepted; media state will be refreshed"}, nil
}

func windowsEjectOperationError(operation, path string, err error) error {
	return &device.OperationError{
		Code:   classifyWindowsEjectError(err),
		Op:     operation,
		Device: device.DeviceRef{Path: path},
		Detail: fmt.Sprintf("native error: %v", err),
		Cause:  err,
	}
}

func classifyWindowsEjectError(err error) device.ErrorCode {
	switch {
	case errors.Is(err, windows.ERROR_ACCESS_DENIED):
		return device.ErrorPermissionDenied
	case errors.Is(err, windows.ERROR_SHARING_VIOLATION),
		errors.Is(err, windows.ERROR_LOCK_VIOLATION),
		errors.Is(err, windows.ERROR_LOCK_FAILED),
		errors.Is(err, windows.ERROR_BUSY),
		errors.Is(err, windows.ERROR_BUSY_DRIVE):
		return device.ErrorBusy
	case errors.Is(err, windows.ERROR_INVALID_FUNCTION),
		errors.Is(err, windows.ERROR_NOT_SUPPORTED):
		return device.ErrorUnsupported
	case errors.Is(err, windows.ERROR_NO_MEDIA_IN_DRIVE),
		errors.Is(err, windows.ERROR_NOT_READY):
		return device.ErrorNoMedia
	case errors.Is(err, windows.ERROR_MEDIA_CHANGED):
		return device.ErrorMediaChanged
	case errors.Is(err, windows.ERROR_FILE_NOT_FOUND),
		errors.Is(err, windows.ERROR_PATH_NOT_FOUND),
		errors.Is(err, windows.ERROR_INVALID_DRIVE),
		errors.Is(err, windows.ERROR_DEVICE_NOT_CONNECTED),
		errors.Is(err, windows.ERROR_NO_SUCH_DEVICE):
		return device.ErrorDeviceRemoved
	default:
		return device.ErrorDeviceFailure
	}
}

func ejectHostCapability(path string) device.Capability {
	return device.Capability{Status: device.SupportSupported, Detail: "Windows storage eject ioctl"}
}
