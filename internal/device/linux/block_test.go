package linux

import "testing"

func TestCanonicalizeOpticalDevicePathAcceptsCanonicalSRPath(t *testing.T) {
	got, err := CanonicalizeOpticalDevicePath("/dev/sr0")
	if err != nil {
		t.Fatalf("canonicalize optical device path: %v", err)
	}
	if got != "/dev/sr0" {
		t.Fatalf("unexpected canonical path: %q", got)
	}
}

func TestCanonicalizeOpticalDevicePathRejectsNonOpticalPath(t *testing.T) {
	if _, err := CanonicalizeOpticalDevicePath("/dev/sda"); err == nil {
		t.Fatal("expected non-optical path to fail")
	}
}

func TestBlockOpenOptionsRequireReadOnlyMode(t *testing.T) {
	options := BlockOpenOptions{
		Path: "/dev/sr1",
		Mode: OpenMode("read_write"),
	}
	if err := options.Validate(); err == nil {
		t.Fatal("expected non-read-only mode to fail")
	}
}

func TestProbeMetadataRejectsOversizedProfile(t *testing.T) {
	probe := ProbeMetadata{
		DevicePath:        "/dev/sr0",
		LogicalSectorSize: 2048,
		CapacitySectors:   1024,
		Profile:           "abcdefghijklmnopqrstuvwxyz1234567",
	}
	if err := probe.Validate(); err == nil {
		t.Fatal("expected oversized profile to fail")
	}
}

func TestBuildBlockAdapterProducesBoundedMediaInfo(t *testing.T) {
	adapter, err := BuildBlockAdapter(
		BlockOpenOptions{
			Path:                "/dev/sr0",
			Mode:                OpenModeReadOnly,
			RequireOptical:      true,
			RequireAdvisoryLock: true,
		},
		ProbeMetadata{
			DevicePath:        "/dev/sr0",
			LogicalSectorSize: 2048,
			CapacitySectors:   8192,
			Profile:           "dvd-rom",
		},
	)
	if err != nil {
		t.Fatalf("build block adapter: %v", err)
	}
	info := adapter.Probe.MediaInfo()
	if info.LogicalSectorSize != 2048 || info.CapacitySectors != 8192 || info.Profile != "dvd-rom" {
		t.Fatalf("unexpected media info: %+v", info)
	}
}
