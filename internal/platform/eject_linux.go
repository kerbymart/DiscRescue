//go:build linux

package platform

import (
	"fmt"
	"os"
	"syscall"

	"discrescue/internal/device"
)

const (
	linuxCDROMEject    = 0x5309
	linuxCDROMLockdoor = 0x5329
)

func ejectHostOpticalDrive(path string, request device.EjectRequest) (device.EjectResult, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorPermissionDenied, Op: "open drive for eject", Device: device.DeviceRef{Path: path}, Cause: err}
	}
	defer file.Close()
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), linuxCDROMLockdoor, 0); errno != 0 {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorDeviceFailure, Op: "unlock drive door", Device: device.DeviceRef{Path: path}, Detail: errno.Error(), Cause: errno}
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, file.Fd(), linuxCDROMEject, 0); errno != 0 {
		return device.EjectResult{Requested: request}, &device.OperationError{Code: device.ErrorDeviceFailure, Op: "eject optical drive", Device: device.DeviceRef{Path: path}, Detail: errno.Error(), Cause: errno}
	}
	return device.EjectResult{Requested: request, Status: device.OperationAccepted, Verification: device.EjectAcceptedUnverified, Detail: fmt.Sprintf("native eject accepted for %s; media state will be refreshed", path)}, nil
}

func ejectHostCapability(path string) device.Capability {
	return device.Capability{Status: device.SupportSupported, Detail: "Linux optical drive ioctl eject"}
}
