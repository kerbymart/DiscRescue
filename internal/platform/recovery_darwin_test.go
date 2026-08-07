//go:build darwin

package platform

import (
	"path/filepath"
	"testing"
)

func TestDarwinRecoveryTargetCanStartNew(t *testing.T) {
	output := filepath.Join(t.TempDir(), "capture.iso")
	status, err := (OSRecovery{}).InspectRecoveryTarget(RecoveryInput{
		OutputPath:        output,
		LogicalSectorSize: 2048,
		CapacitySectors:   32,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !status.CanStartNew || status.MapPath != filepath.Join(filepath.Dir(output), "capture.drmap") {
		t.Fatalf("unexpected target status: %+v", status)
	}
}
