package catalog

import (
	"crypto/sha256"
	"testing"
	"time"
)

func TestIdentityRecordSameLogicalContentIgnoresProvenanceDifferences(t *testing.T) {
	content := ContentIdentity{
		Version:          1,
		Profile:          0x10,
		LogicalBlockSize: 2048,
		SectorCount:      4096,
		Sessions:         1,
		Tracks: []TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: 1, LeadOutLBA: 4096},
		},
		LayoutSHA256: "layout-a",
		VolumeHints:  []VolumeHint{{HintType: 1, Value: "DISC_ONE"}},
		Samples: []SectorFingerprint{
			{Slot: 1, LBA: 0, Available: true, SHA256: sha256.Sum256([]byte("sample-1"))},
		},
		QuickID: "quick-a",
	}

	left := IdentityRecord{
		Content: content,
		Capture: CaptureIdentity{
			CaptureID: "capture-1",
			Device:    DeviceIdentity{Vendor: "PLEXTOR", Product: "PX-760A", Serial: "A1", Transport: "sata"},
			StartedAt: time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
			UserLabel: "First capture",
			PhysicalCopy: &PhysicalCopyIdentity{
				AssetID: "SHELF-001",
			},
		},
	}
	right := IdentityRecord{
		Content: content,
		Capture: CaptureIdentity{
			CaptureID: "capture-2",
			Device:    DeviceIdentity{Vendor: "ASUS", Product: "BW-16D1HT", Serial: "B2", Transport: "usb"},
			StartedAt: time.Date(2026, time.August, 5, 11, 0, 0, 0, time.UTC),
			UserLabel: "Second capture",
			PhysicalCopy: &PhysicalCopyIdentity{
				AssetID:     "SHELF-002",
				HubCodeNote: "hub-code-note",
			},
		},
	}

	if err := left.Validate(); err != nil {
		t.Fatalf("validate left identity record: %v", err)
	}
	if err := right.Validate(); err != nil {
		t.Fatalf("validate right identity record: %v", err)
	}
	if !left.SameLogicalContent(right) {
		t.Fatal("expected logical-content match to ignore capture, device, and physical-copy provenance")
	}
}

func TestIdentityRecordSameLogicalContentChangesWhenLogicalContentChanges(t *testing.T) {
	left := IdentityRecord{
		Content: ContentIdentity{
			Version:          1,
			Profile:          0x10,
			LogicalBlockSize: 2048,
			SectorCount:      4096,
			Sessions:         1,
			Tracks: []TrackLayout{
				{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: 1, LeadOutLBA: 4096},
			},
			LayoutSHA256: "layout-a",
			Samples: []SectorFingerprint{
				{Slot: 1, LBA: 0, Available: true, SHA256: sha256.Sum256([]byte("sample-a"))},
			},
			QuickID: "quick-a",
		},
		Capture: CaptureIdentity{
			CaptureID: "capture-1",
			Device:    DeviceIdentity{Vendor: "PLEXTOR", Product: "PX-760A"},
			StartedAt: time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
		},
	}
	right := left
	right.Content.LayoutSHA256 = "layout-b"

	if left.SameLogicalContent(right) {
		t.Fatal("expected logical-content match to change when content identity changes")
	}
}

func TestCaptureIdentityRequiresSeparateProvenanceFields(t *testing.T) {
	capture := CaptureIdentity{
		CaptureID: "capture-1",
		Device:    DeviceIdentity{Vendor: "PLEXTOR", Product: "PX-760A"},
		StartedAt: time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
	}
	if err := capture.Validate(); err != nil {
		t.Fatalf("validate capture identity: %v", err)
	}

	if err := (CaptureIdentity{CaptureID: "capture-2"}).Validate(); err == nil {
		t.Fatal("expected capture identity without device or start time to fail")
	}
}

func TestPhysicalCopyIdentityIsOptionalButValidatedWhenPresent(t *testing.T) {
	record := IdentityRecord{
		Content: ContentIdentity{
			Version:          1,
			Profile:          0x10,
			LogicalBlockSize: 2048,
			SectorCount:      4096,
			Sessions:         1,
			Tracks: []TrackLayout{
				{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: 1, LeadOutLBA: 4096},
			},
			LayoutSHA256: "layout-a",
			Samples: []SectorFingerprint{
				{Slot: 1, LBA: 0, Available: true, SHA256: sha256.Sum256([]byte("sample-a"))},
			},
		},
		Capture: CaptureIdentity{
			CaptureID: "capture-1",
			Device:    DeviceIdentity{Vendor: "PLEXTOR", Product: "PX-760A"},
			StartedAt: time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
		},
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("validate identity record without physical copy: %v", err)
	}

	record.Capture.PhysicalCopy = &PhysicalCopyIdentity{}
	if err := record.Validate(); err == nil {
		t.Fatal("expected empty physical-copy identity to fail validation")
	}
}
