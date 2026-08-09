//go:build darwin

package platform

import "testing"

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
