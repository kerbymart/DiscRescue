package catalog

import (
	"crypto/sha256"
	"testing"
)

func TestBuildLayoutHashIsDeterministicAndIncludesVolumeHints(t *testing.T) {
	base := ContentIdentity{
		Version:          1,
		Profile:          0x10,
		LogicalBlockSize: 2048,
		SectorCount:      4096,
		Sessions:         1,
		Tracks: []TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: 1, LeadOutLBA: 4096},
		},
		VolumeHints: []VolumeHint{
			{HintType: 2, Value: "DISC_A"},
			{HintType: 1, Value: "VOL_A"},
		},
	}

	left, err := BuildLayoutHash(base)
	if err != nil {
		t.Fatalf("build layout hash: %v", err)
	}
	right, err := BuildLayoutHash(base)
	if err != nil {
		t.Fatalf("build layout hash: %v", err)
	}
	if left != right {
		t.Fatalf("expected deterministic layout hash, got %q and %q", left, right)
	}

	changed := base
	changed.VolumeHints = []VolumeHint{{HintType: 1, Value: "VOL_B"}}
	different, err := BuildLayoutHash(changed)
	if err != nil {
		t.Fatalf("build layout hash: %v", err)
	}
	if left == different {
		t.Fatal("expected layout hash to change when canonical content-layout inputs change")
	}
}

func TestBuildQuickContentIDRequiresAllMandatorySlots(t *testing.T) {
	identity := ContentIdentity{
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
			{Slot: 1, LBA: 0, Available: true, SHA256: sha256.Sum256([]byte("sample-1"))},
		},
	}

	quickID, ok, err := BuildQuickContentID(identity, []uint16{1, 2})
	if err != nil {
		t.Fatalf("build quick content id: %v", err)
	}
	if ok || quickID != "" {
		t.Fatalf("expected missing mandatory slot to suppress quick id, got ok=%v quickID=%q", ok, quickID)
	}
}

func TestBuildQuickContentIDIsDeterministicAcrossSlotOrder(t *testing.T) {
	identity := ContentIdentity{
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
			{Slot: 2, LBA: 16, Available: true, SHA256: sha256.Sum256([]byte("sample-2"))},
			{Slot: 1, LBA: 0, Available: true, SHA256: sha256.Sum256([]byte("sample-1"))},
		},
	}

	left, ok, err := BuildQuickContentID(identity, []uint16{1, 2})
	if err != nil {
		t.Fatalf("build quick content id: %v", err)
	}
	if !ok {
		t.Fatal("expected quick id to be present")
	}
	right, ok, err := BuildQuickContentID(identity, []uint16{2, 1})
	if err != nil {
		t.Fatalf("build quick content id: %v", err)
	}
	if !ok || left != right {
		t.Fatalf("expected deterministic quick id, got left=%q right=%q ok=%v", left, right, ok)
	}
}

func TestCompareContentIdentityProbableAndStrongAndConflict(t *testing.T) {
	candidate := identityWithSamplesAndQuick("layout-x", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
		availableSample(2, 16, "same-2"),
		availableSample(3, 64, "same-3"),
		availableSample(4, 128, "same-4"),
	}, "")

	probableObserved := identityWithSamplesAndQuick("layout-x", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
		availableSample(2, 16, "same-2"),
		availableSample(9, 512, "different-slot"),
	}, "")
	probable, err := CompareContentIdentity(candidate, probableObserved, 8)
	if err != nil {
		t.Fatalf("compare probable: %v", err)
	}
	if probable.Match != MatchProbable || probable.MatchingSamples != 2 {
		t.Fatalf("unexpected probable result: %+v", probable)
	}

	strongObserved := identityWithSamplesAndQuick("layout-x", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
		availableSample(2, 16, "same-2"),
		availableSample(3, 64, "same-3"),
		availableSample(4, 128, "same-4"),
	}, "")
	strong, err := CompareContentIdentity(candidate, strongObserved, 8)
	if err != nil {
		t.Fatalf("compare strong: %v", err)
	}
	if strong.Match != MatchStrong || strong.MatchingSamples != 4 {
		t.Fatalf("unexpected strong result: %+v", strong)
	}

	conflictObserved := identityWithSamplesAndQuick("layout-x", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
		availableSample(2, 16, "conflict-2"),
	}, "")
	conflict, err := CompareContentIdentity(candidate, conflictObserved, 8)
	if err != nil {
		t.Fatalf("compare conflict: %v", err)
	}
	if conflict.Match != MatchConflict || conflict.ConflictingSamples != 1 {
		t.Fatalf("unexpected conflict result: %+v", conflict)
	}
}

func TestCompareContentIdentityReturnsNoMatchForLayoutMismatch(t *testing.T) {
	left := identityWithSamplesAndQuick("layout-left", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
	}, "")
	right := identityWithSamplesAndQuick("layout-right", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
	}, "")

	result, err := CompareContentIdentity(left, right, 8)
	if err != nil {
		t.Fatalf("compare no match: %v", err)
	}
	if result.Match != MatchNo {
		t.Fatalf("unexpected no-match result: %+v", result)
	}
}

func TestCompareContentIdentityReturnsIndeterminateWhenBudgetExhausted(t *testing.T) {
	left := identityWithSamplesAndQuick("layout-budget", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
		availableSample(2, 16, "same-2"),
		availableSample(3, 64, "same-3"),
	}, "")
	right := identityWithSamplesAndQuick("layout-budget", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
		availableSample(2, 16, "same-2"),
		availableSample(3, 64, "same-3"),
	}, "")

	result, err := CompareContentIdentity(left, right, 2)
	if err != nil {
		t.Fatalf("compare indeterminate: %v", err)
	}
	if result.Match != MatchIndeterminate || !result.BudgetExhausted {
		t.Fatalf("unexpected indeterminate result: %+v", result)
	}
}

func identityWithSamplesAndQuick(layout string, samples []SectorFingerprint, quickID string) ContentIdentity {
	return ContentIdentity{
		Version:          1,
		Profile:          0x10,
		LogicalBlockSize: 2048,
		SectorCount:      4096,
		Sessions:         1,
		Tracks: []TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 4095, Mode: 1, LeadOutLBA: 4096},
		},
		LayoutSHA256: layout,
		Samples:      samples,
		QuickID:      quickID,
	}
}
