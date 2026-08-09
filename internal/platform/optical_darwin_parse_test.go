package platform

import "testing"

func TestNormalizeDarwinOpticalDevice(t *testing.T) {
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{"/dev/disk4", "/dev/rdisk4", true},
		{"/dev/rdisk4", "/dev/rdisk4", true},
		{"/dev/disk4s1", "", false},
		{"/dev/sr0", "", false},
	}
	for _, test := range tests {
		got, err := normalizeDarwinOpticalDevice(test.path)
		if test.ok && (err != nil || got != test.want) {
			t.Errorf("normalize %q: got %q, %v; want %q", test.path, got, err, test.want)
		}
		if !test.ok && err == nil {
			t.Errorf("normalize %q: expected error", test.path)
		}
	}
}

func TestParseDarwinDiskutilList(t *testing.T) {
	got := parseDarwinDiskutilList("/dev/disk2 (external, physical):\n   0: CD_partition_scheme *700.0 MB disk2\n/dev/disk3 (internal, physical):\n")
	if len(got) != 2 || got[0].Path != "/dev/disk2" || got[1].Path != "/dev/disk3" {
		t.Fatalf("unexpected drives: %+v", got)
	}
}

func TestParseDarwinDiskutilInfo(t *testing.T) {
	got, err := parseDarwinDiskutilInfo("Device Node: /dev/disk2\nOptical Media: Yes\nMedia Name: CD-ROM\nFile System Personality: ISO 9660\nVolume Name: SAMPLE\nDevice Block Size: 2048 Bytes\nDisk Size: 700.0 MB (734003200 Bytes)\n")
	if err != nil {
		t.Fatal(err)
	}
	if got.DevicePath != "/dev/disk2" || !got.OpticalMedia || got.LogicalSectorSize != 2048 || got.CapacityBytes != 734003200 || got.VolumeName != "SAMPLE" {
		t.Fatalf("unexpected info: %+v", got)
	}
}

func TestParseDarwinDiskutilInfoRecognizesOpticalMediaType(t *testing.T) {
	got, err := parseDarwinDiskutilInfo("Device Node: /dev/disk7\nDevice / Media Name: USB DVD Writer\nVolume Name: TEST-VOLUME\nFile System Personality: ISO Joliet\nDisk Size: 4.6 GB (4649582592 Bytes)\nDevice Block Size: 2048 Bytes\nOptical Drive Type: CD-ROM, DVD-ROM\nOptical Media Type: DVD-R\n")
	if err != nil {
		t.Fatal(err)
	}
	if !got.OpticalMedia || got.VolumeName != "TEST-VOLUME" || got.CapacityBytes != 4649582592 {
		t.Fatalf("unexpected optical media info: %+v", got)
	}
}

func TestDarwinDriveDisplayNameUsesLabelAndTechnicalPath(t *testing.T) {
	got := darwinDriveDisplayName(darwinDiskInfo{VolumeName: "ARCHIVE"}, "/dev/disk4")
	if got != "ARCHIVE (/dev/disk4)" {
		t.Fatalf("unexpected display name: %q", got)
	}
}

func TestParseDarwinDiskutilInfoKeepsOpticalDriveWithoutMediaGeometry(t *testing.T) {
	got, err := parseDarwinDiskutilInfo("Device Node: /dev/disk4\nDevice / Media Name: USB DVD Writer\nOptical Drive Type: CD-ROM\n")
	if err != nil {
		t.Fatal(err)
	}
	if !got.OpticalMedia || got.LogicalSectorSize != 0 || got.CapacityBytes != 0 {
		t.Fatalf("unexpected blank-drive info: %+v", got)
	}
}

func TestParseDarwinDrutilList(t *testing.T) {
	got := parseDarwinDrutilList("   Vendor   Product           Rev   Bus       SupportLevel\n1  TS8XDVDS TRANSCEND         1.03  USB       Unsupported\n")
	if len(got) != 1 {
		t.Fatalf("got %d drives: %+v", len(got), got)
	}
	if got[0].Index != 1 || got[0].Vendor != "TS8XDVDS" || got[0].Product != "TRANSCEND" || got[0].Bus != "USB" || got[0].SupportLevel != "Unsupported" {
		t.Fatalf("unexpected drutil drive: %+v", got[0])
	}
	path := drutilDrivePath(got[0].Index)
	if path != "drutil:1" {
		t.Fatalf("unexpected synthetic path %q", path)
	}
	if index, ok := parseDrutilDrivePath(path); !ok || index != 1 {
		t.Fatalf("failed to parse synthetic path")
	}
}
