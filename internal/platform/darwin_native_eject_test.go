//go:build darwin

package platform

import "testing"

func TestDarwinForceEjectNormalizesToRawDevice(t *testing.T) {
	got, err := normalizeDarwinOpticalDevice("/dev/disk4")
	if err != nil {
		t.Fatalf("normalize device: %v", err)
	}
	if got != "/dev/rdisk4" {
		t.Fatalf("normalized path = %q, want /dev/rdisk4", got)
	}
}
