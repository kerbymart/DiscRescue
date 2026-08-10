//go:build darwin

package platform

import (
	"errors"
	"syscall"
	"testing"
)

func TestDarwinDeviceCandidatesUseConfiguredPureGoPaths(t *testing.T) {
	t.Setenv("DISKRESCUE_DARWIN_OPTICAL_DEVICES", "/dev/rdisk4, /dev/rdisk5")
	paths, err := darwinDeviceCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != "/dev/rdisk4" || paths[1] != "/dev/rdisk5" {
		t.Fatalf("unexpected candidates: %#v", paths)
	}
}

func TestDarwinDeviceCandidatesIgnoreEmptyConfiguredEntries(t *testing.T) {
	t.Setenv("DISKRESCUE_DARWIN_OPTICAL_DEVICES", ", /dev/rdisk4, ,")
	paths, err := darwinDeviceCandidates()
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != "/dev/rdisk4" {
		t.Fatalf("unexpected candidates: %#v", paths)
	}
}

func TestDarwinProbeErrorClassifiesNativeFailures(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		state MediaProbeState
	}{
		{name: "missing node", cause: syscall.ENOENT, state: MediaProbeUnavailable},
		{name: "not ready", cause: syscall.EIO, state: MediaProbeNotReady},
		{name: "permission", cause: syscall.EACCES, state: MediaProbePermission},
		{name: "busy", cause: syscall.EBUSY, state: MediaProbeBusy},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := darwinProbeError("/dev/rdisk4", "open", test.cause)
			var probeErr *MediaInspectionError
			if !errors.As(err, &probeErr) || probeErr.State != test.state {
				t.Fatalf("got %#v; want state %q", err, test.state)
			}
		})
	}
}

func TestDarwinDriveStatusKeepsKnownDriveWhenMediaIsAbsent(t *testing.T) {
	status := darwinDriveStatus(darwinNativeDrive{
		Path:  "/dev/rdisk4",
		State: MediaProbeNoMedia,
	})
	if status != "drive present; no media" {
		t.Fatalf("unexpected status: %q", status)
	}
}

func TestAutomaticDarwinDiscoverySkipsUnverifiedCandidates(t *testing.T) {
	for _, probeErr := range []error{
		darwinProbeError("/dev/rdisk0", "open", syscall.EACCES),
		darwinProbeError("/dev/rdisk1", "DKIOCGETBLOCKSIZE", syscall.EIO),
	} {
		if shouldRetainDarwinCandidate(false, probeErr) {
			t.Fatalf("automatic discovery retained unverified candidate: %v", probeErr)
		}
	}
}

func TestConfiguredDarwinOpticalDeviceRetainsActionableProbeFailure(t *testing.T) {
	err := darwinProbeError("/dev/rdisk4", "open", syscall.EACCES)
	if !shouldRetainDarwinCandidate(true, err) {
		t.Fatal("configured optical device lost its actionable access failure")
	}
}

func TestDarwinDiscoveryRetainsSuccessfulNoMediaProbe(t *testing.T) {
	if !shouldRetainDarwinCandidate(false, nil) {
		t.Fatal("successful no-media probe was not retained")
	}
}
