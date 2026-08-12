//go:build windows

package platform

import (
	"errors"
	"strings"
	"testing"

	"discrescue/internal/device"
	"golang.org/x/sys/windows"
)

func TestRawVolumePathNormalizesWindowsDriveRoots(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "drive root", path: `D:\`, want: `\\.\D:`},
		{name: "drive name", path: `D:`, want: `\\.\D:`},
		{name: "already normalized", path: `\\.\D:`, want: `\\.\D:`},
		{name: "normalized with trailing slash", path: `\\.\D:\`, want: `\\.\D:`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rawVolumePath(test.path); got != test.want {
				t.Fatalf("rawVolumePath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

func TestClassifyWindowsEjectError(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		want  device.ErrorCode
	}{
		{name: "permission", cause: windows.ERROR_ACCESS_DENIED, want: device.ErrorPermissionDenied},
		{name: "sharing conflict", cause: windows.ERROR_SHARING_VIOLATION, want: device.ErrorBusy},
		{name: "unsupported ioctl", cause: windows.ERROR_INVALID_FUNCTION, want: device.ErrorUnsupported},
		{name: "no media", cause: windows.ERROR_NO_MEDIA_IN_DRIVE, want: device.ErrorNoMedia},
		{name: "media changed", cause: windows.ERROR_MEDIA_CHANGED, want: device.ErrorMediaChanged},
		{name: "removed drive", cause: windows.ERROR_DEVICE_NOT_CONNECTED, want: device.ErrorDeviceRemoved},
		{name: "other native failure", cause: windows.ERROR_GEN_FAILURE, want: device.ErrorDeviceFailure},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyWindowsEjectError(test.cause); got != test.want {
				t.Fatalf("classifyWindowsEjectError(%v) = %q, want %q", test.cause, got, test.want)
			}
		})
	}
}

func TestWindowsEjectOperationErrorPreservesNativeCause(t *testing.T) {
	nativeErr := windows.ERROR_ACCESS_DENIED
	err := windowsEjectOperationError("open drive for eject", `D:\`, nativeErr)
	if !device.IsCode(err, device.ErrorPermissionDenied) {
		t.Fatalf("error code = %v, want permission denied: %v", err, err)
	}
	if !errors.Is(err, nativeErr) {
		t.Fatalf("error %v does not preserve native cause %v", err, nativeErr)
	}
	if !strings.Contains(err.Error(), nativeErr.Error()) {
		t.Fatalf("error %q does not preserve native detail", err)
	}
}
