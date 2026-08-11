//go:build darwin

package platform

import (
	"os"
	"testing"

	"discrescue/internal/device"
)

// TestDarwinNormalEjectIntegration is intentionally opt-in because it ejects
// the supplied removable disc through the same adapter the TUI uses.
func TestDarwinNormalEjectIntegration(t *testing.T) {
	path := os.Getenv("DISKRESCUE_DARWIN_EJECT_TEST_DEVICE")
	if path == "" {
		t.Skip("set DISKRESCUE_DARWIN_EJECT_TEST_DEVICE to eject a disposable optical disc")
	}
	result, err := ejectHostOpticalDrive(path, device.EjectRequest{Mode: device.EjectNormal})
	if err != nil {
		t.Fatalf("normal eject %s: %v", path, err)
	}
	if result.Status != device.OperationAccepted {
		t.Fatalf("normal eject status = %s, want %s", result.Status, device.OperationAccepted)
	}
}
