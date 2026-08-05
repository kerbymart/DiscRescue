package merge

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"discrescue/internal/catalog"
	"discrescue/internal/mapfile"
)

func TestBuildPlanBlocksLogicalContentConflict(t *testing.T) {
	_, err := BuildPlan([]Capture{
		{
			CaptureID: "capture-a",
			Identity:  mergeIdentity("layout-a"),
		},
		{
			CaptureID: "capture-b",
			Identity:  mergeIdentity("layout-b"),
		},
	})
	if err == nil {
		t.Fatal("expected logical-content identity conflict to block automatic merge")
	}
}

func TestBuildPlanRejectsOverlappingExtentsInsideCapture(t *testing.T) {
	_, err := BuildPlan([]Capture{{
		CaptureID: "capture-a",
		Identity:  mergeIdentity("layout-a"),
		Extents: []mapfile.Extent{
			mergeExtent(0, 2, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, "a"),
			mergeExtent(1, 2, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, "b"),
		},
	}})
	if err == nil {
		t.Fatal("expected overlapping extents to fail validation")
	}
}

func TestBuildPlanPrefersTrustedChecksumMatches(t *testing.T) {
	plan, err := BuildPlan([]Capture{
		{
			CaptureID: "capture-trusted",
			Identity:  mergeIdentity("layout-a"),
			Extents: []mapfile.Extent{
				mergeExtent(0, 2, mapfile.SectorStateVerified, mapfile.ConfidenceTrustedChecksum, "same"),
			},
		},
		{
			CaptureID: "capture-unverified",
			Identity:  mergeIdentity("layout-a"),
			Extents: []mapfile.Extent{
				mergeExtent(0, 2, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, "same"),
			},
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	if len(plan.Extents) != 2 {
		t.Fatalf("unexpected merged extent count: %+v", plan.Extents)
	}
	got := plan.Extents[0]
	if got.SelectedCaptureID != "capture-trusted" || got.SelectionRule != RuleTrustedChecksumMatch {
		t.Fatalf("unexpected trusted selection: %+v", got)
	}
	if got.Extent.State != mapfile.SectorStateVerified || got.Extent.Confidence != mapfile.ConfidenceTrustedChecksum {
		t.Fatalf("unexpected trusted extent: %+v", got.Extent)
	}
	if plan.Extents[1].SelectionRule != RuleMissing {
		t.Fatalf("expected uncovered tail to remain missing, got %+v", plan.Extents[1])
	}
}

func TestBuildPlanPromotesIdenticalUnverifiedIndependentCaptures(t *testing.T) {
	plan, err := BuildPlan([]Capture{
		{
			CaptureID: "capture-a",
			Identity:  mergeIdentity("layout-a"),
			Extents: []mapfile.Extent{
				mergeExtent(0, 3, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, "same"),
			},
		},
		{
			CaptureID: "capture-b",
			Identity:  mergeIdentity("layout-a"),
			Extents: []mapfile.Extent{
				mergeExtent(0, 3, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, "same"),
			},
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	got := plan.Extents[0]
	if got.SelectionRule != RuleIdenticalUnverifiedCandidates {
		t.Fatalf("unexpected rule: %+v", got)
	}
	if got.Extent.State != mapfile.SectorStateVerified || got.Extent.Confidence != mapfile.ConfidenceRepeatedIndependentCapture {
		t.Fatalf("unexpected promoted extent: %+v", got.Extent)
	}
}

func TestBuildPlanMarksConflictingVerifiedCandidates(t *testing.T) {
	plan, err := BuildPlan([]Capture{
		{
			CaptureID: "capture-a",
			Identity:  mergeIdentity("layout-a"),
			Extents: []mapfile.Extent{
				mergeExtent(0, 1, mapfile.SectorStateVerified, mapfile.ConfidenceRepeatedSingleCapture, "left"),
			},
		},
		{
			CaptureID: "capture-b",
			Identity:  mergeIdentity("layout-a"),
			Extents: []mapfile.Extent{
				mergeExtent(0, 1, mapfile.SectorStateVerified, mapfile.ConfidenceRepeatedSingleCapture, "right"),
			},
		},
	})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	got := plan.Extents[0]
	if got.SelectionRule != RuleConflict || !got.UnresolvedConflict {
		t.Fatalf("unexpected conflict result: %+v", got)
	}
	if got.Extent.State != mapfile.SectorStateConflicting || got.Extent.Confidence != mapfile.ConfidenceNone {
		t.Fatalf("unexpected conflicting extent: %+v", got.Extent)
	}
	if len(got.CandidateHashes) != 2 {
		t.Fatalf("expected both candidate hashes to be recorded, got %+v", got.CandidateHashes)
	}
}

func TestBuildPlanFillsMissingWhenNoCaptureCoversSector(t *testing.T) {
	plan, err := BuildPlan([]Capture{{
		CaptureID: "capture-a",
		Identity:  mergeIdentity("layout-a"),
		Extents: []mapfile.Extent{
			mergeExtent(1, 1, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, "only-middle"),
		},
	}})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	if len(plan.Extents) != 3 {
		t.Fatalf("expected leading missing, covered, trailing missing extents, got %+v", plan.Extents)
	}
	if plan.Extents[0].SelectionRule != RuleMissing || plan.Extents[2].SelectionRule != RuleMissing {
		t.Fatalf("expected missing gaps to remain explicit, got %+v", plan.Extents)
	}
}

func mergeIdentity(layout string) catalog.ContentIdentity {
	return catalog.ContentIdentity{
		Version:          1,
		Profile:          0x10,
		LogicalBlockSize: 2048,
		SectorCount:      4,
		Sessions:         1,
		Tracks: []catalog.TrackLayout{
			{TrackNumber: 1, StartLBA: 0, EndLBA: 3, Mode: 1, LeadOutLBA: 4},
		},
		LayoutSHA256: layout,
	}
}

func mergeExtent(start uint64, sectors uint32, state mapfile.SectorState, confidence mapfile.Confidence, marker string) mapfile.Extent {
	sum := sha256.Sum256([]byte(marker))
	var hash [16]byte
	copy(hash[:], sum[:16])
	return mapfile.Extent{
		StartLBA:   start,
		Sectors:    sectors,
		State:      state,
		Confidence: confidence,
		DataHash:   hash,
	}
}

func TestCanMergeMergedExtentRequiresMatchingMetadata(t *testing.T) {
	left := MergedExtent{
		Extent: mapfile.Extent{
			StartLBA:   0,
			Sectors:    1,
			State:      mapfile.SectorStateVerified,
			Confidence: mapfile.ConfidenceTrustedChecksum,
		},
		SelectedCaptureID: "capture-a",
		SelectionRule:     RuleTrustedChecksumMatch,
		CandidateHashes:   [][16]byte{{1}},
	}
	right := left
	right.Extent.StartLBA = 1

	if !canMergeMergedExtent(left, right) {
		t.Fatal("expected adjacent merged extents with matching metadata to coalesce")
	}

	right.CandidateHashes = [][16]byte{{2}}
	if canMergeMergedExtent(left, right) {
		t.Fatal("expected differing candidate hashes to block coalescing")
	}
}

func TestUniqueBoundariesSortsAndDeduplicates(t *testing.T) {
	got := uniqueBoundaries([]uint64{0, 2, 2, 4, 4})
	want := []uint64{0, 2, 4}
	if !bytes.Equal(uint64SliceBytes(got), uint64SliceBytes(want)) {
		t.Fatalf("unexpected boundaries: got=%v want=%v", got, want)
	}
}

func uint64SliceBytes(values []uint64) []byte {
	out := make([]byte, 0, len(values)*8)
	for _, value := range values {
		out = append(out, byte(value))
	}
	return out
}
