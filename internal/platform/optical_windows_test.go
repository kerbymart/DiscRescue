//go:build windows

package platform

import (
	"errors"
	"testing"

	"discrescue/internal/device"
	"golang.org/x/sys/windows"
)

func TestWindowsMediaProbeErrorClassifiesNativeFailures(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		state MediaProbeState
	}{
		{name: "empty drive", cause: windows.ERROR_NO_MEDIA_IN_DRIVE, state: MediaProbeNoMedia},
		{name: "not ready", cause: windows.ERROR_NOT_READY, state: MediaProbeNotReady},
		{name: "permission", cause: windows.ERROR_ACCESS_DENIED, state: MediaProbePermission},
		{name: "busy", cause: windows.ERROR_BUSY, state: MediaProbeBusy},
		{name: "removed", cause: windows.ERROR_DEVICE_NOT_CONNECTED, state: MediaProbeUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := windowsMediaProbeError(`D:\`, test.cause)
			var probeErr *MediaInspectionError
			if !errors.As(err, &probeErr) || probeErr.State != test.state {
				t.Fatalf("got %#v; want state %q", err, test.state)
			}
		})
	}
}

func TestWindowsCapabilitiesDoNotAdvertiseForceEject(t *testing.T) {
	capabilities := hostOpticalCapabilities(`D:\`)
	if capabilities.NormalEject.Status != device.SupportSupported {
		t.Fatalf("normal eject capability = %+v, want supported", capabilities.NormalEject)
	}
	if capabilities.ForceEject.Status != device.SupportUnsupported {
		t.Fatalf("force eject capability = %+v, want unsupported", capabilities.ForceEject)
	}
}
