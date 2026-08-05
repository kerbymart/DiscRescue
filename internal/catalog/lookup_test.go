package catalog

import (
	"crypto/sha256"
	"testing"
)

func TestLookupReturnsStrongForQuickIDMatch(t *testing.T) {
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
		QuickID:      "quick-a",
	}
	entry := Entry{Identity: identity, Status: string(ProcessingObserved)}

	result, err := Lookup([]Entry{entry}, identity, LookupBudget{MaxComparedSamples: 4})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if result.Match != MatchStrong {
		t.Fatalf("unexpected match strength: %s", result.Match)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("unexpected candidate count: %d", len(result.Candidates))
	}
}

func TestLookupReturnsConflictForOverlappingSampleMismatch(t *testing.T) {
	left := identityWithSamples("layout-b", []SectorFingerprint{
		availableSample(1, 0, "left"),
		availableSample(2, 16, "left-2"),
	})
	right := identityWithSamples("layout-b", []SectorFingerprint{
		availableSample(1, 0, "left"),
		availableSample(2, 16, "right-2"),
	})

	result, err := Lookup([]Entry{{Identity: left, Status: string(ProcessingObserved)}}, right, LookupBudget{MaxComparedSamples: 4})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if result.Match != MatchConflict {
		t.Fatalf("unexpected match strength: %s", result.Match)
	}
	if result.ConflictingSamples != 1 {
		t.Fatalf("unexpected conflicting sample count: %d", result.ConflictingSamples)
	}
}

func TestLookupReturnsIndeterminateWhenBudgetExhaustsBeforeDecision(t *testing.T) {
	left := identityWithSamples("layout-c", []SectorFingerprint{
		availableSample(1, 0, "same-1"),
		availableSample(2, 16, "same-2"),
		availableSample(3, 64, "same-3"),
		availableSample(4, 128, "same-4"),
	})

	result, err := Lookup([]Entry{{Identity: left, Status: string(ProcessingObserved)}}, left, LookupBudget{MaxComparedSamples: 2})
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if result.Match != MatchIndeterminate {
		t.Fatalf("unexpected match strength: %s", result.Match)
	}
	if !result.BudgetExhausted {
		t.Fatal("expected budget exhaustion to be reported")
	}
}

func identityWithSamples(layout string, samples []SectorFingerprint) ContentIdentity {
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
	}
}

func availableSample(slot uint16, lba int64, marker string) SectorFingerprint {
	return SectorFingerprint{
		Slot:      slot,
		LBA:       lba,
		Available: true,
		SHA256:    sha256.Sum256([]byte(marker)),
	}
}
