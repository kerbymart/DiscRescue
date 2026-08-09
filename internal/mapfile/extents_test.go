package mapfile

import (
	"math"
	"testing"
)

func TestExtentValidateRejectsZeroSectors(t *testing.T) {
	err := (Extent{StartLBA: 10, Sectors: 0, State: SectorStateUnknown, Confidence: ConfidenceNone}).Validate()
	if err == nil {
		t.Fatal("expected zero-sector extent to fail validation")
	}
}

func TestExtentValidateRejectsEndOverflow(t *testing.T) {
	err := (Extent{StartLBA: math.MaxUint64, Sectors: 1}).Validate()
	if err == nil {
		t.Fatal("expected extent end overflow to fail validation")
	}
}

func TestValidateExtentSetWithinCapacity(t *testing.T) {
	valid := []Extent{{StartLBA: 8, Sectors: 2, State: SectorStateMissing, Confidence: ConfidenceNone}}
	if err := ValidateExtentSetWithinCapacity(valid, 10); err != nil {
		t.Fatalf("valid extent rejected: %v", err)
	}
	for _, test := range []struct {
		name   string
		extent Extent
	}{
		{name: "starts at capacity", extent: Extent{StartLBA: 10, Sectors: 1, State: SectorStateMissing}},
		{name: "crosses capacity", extent: Extent{StartLBA: 9, Sectors: 2, State: SectorStateMissing}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateExtentSetWithinCapacity([]Extent{test.extent}, 10); err == nil {
				t.Fatal("expected out-of-range extent to fail")
			}
		})
	}
}

func TestCheckedSectorByteOffsetRejectsOverflow(t *testing.T) {
	if _, err := CheckedSectorByteOffset(math.MaxUint64, 2); err == nil {
		t.Fatal("expected byte offset overflow to fail")
	}
	if got, err := CheckedSectorByteOffset(4, 2048); err != nil || got != 8192 {
		t.Fatalf("offset = %d, err = %v", got, err)
	}
}

func TestOverlaps(t *testing.T) {
	left := Extent{StartLBA: 10, Sectors: 4}
	right := Extent{StartLBA: 13, Sectors: 4}

	if !Overlaps(left, right) {
		t.Fatal("expected overlapping extents")
	}
}

func TestExtentValidateRejectsInvalidConfidenceForState(t *testing.T) {
	err := (Extent{
		StartLBA:   10,
		Sectors:    1,
		State:      SectorStateReadUnverified,
		Confidence: ConfidenceTrustedChecksum,
	}).Validate()
	if err == nil {
		t.Fatal("expected invalid state-confidence pair to fail validation")
	}
}

func TestExtentTransitionPromotesUnverifiedReadToVerified(t *testing.T) {
	extent := Extent{
		StartLBA:   100,
		Sectors:    1,
		State:      SectorStateReadUnverified,
		Confidence: ConfidenceSingleRead,
	}

	next, err := extent.Transition(SectorStateVerified, ConfidenceRepeatedSingleCapture)
	if err != nil {
		t.Fatalf("unexpected transition error: %v", err)
	}
	if next.State != SectorStateVerified || next.Confidence != ConfidenceRepeatedSingleCapture {
		t.Fatalf("unexpected transitioned extent: %+v", next)
	}
}

func TestExtentTransitionRejectsVerifiedDemotionToUnverified(t *testing.T) {
	extent := Extent{
		StartLBA:   200,
		Sectors:    1,
		State:      SectorStateVerified,
		Confidence: ConfidenceTrustedChecksum,
	}

	if _, err := extent.Transition(SectorStateReadUnverified, ConfidenceSingleRead); err == nil {
		t.Fatal("expected demotion from verified to unverified to fail")
	}
}

func TestExtentTransitionRejectsConflictingToUnverified(t *testing.T) {
	extent := Extent{
		StartLBA:   300,
		Sectors:    1,
		State:      SectorStateConflicting,
		Confidence: ConfidenceNone,
	}

	if _, err := extent.Transition(SectorStateReadUnverified, ConfidenceSingleRead); err == nil {
		t.Fatal("expected conflicting to unverified transition to fail")
	}
}

func TestSplitExtent(t *testing.T) {
	extent := Extent{
		StartLBA:   100,
		Sectors:    8,
		State:      SectorStateVerified,
		Confidence: ConfidenceRepeatedIndependentCapture,
	}

	left, right, err := SplitExtent(extent, 104)
	if err != nil {
		t.Fatalf("unexpected split error: %v", err)
	}
	if left.StartLBA != 100 || left.Sectors != 4 {
		t.Fatalf("unexpected left extent: %+v", left)
	}
	if right.StartLBA != 104 || right.Sectors != 4 {
		t.Fatalf("unexpected right extent: %+v", right)
	}
}

func TestMergeAdjacent(t *testing.T) {
	left := Extent{
		StartLBA:   100,
		Sectors:    4,
		State:      SectorStateVerified,
		Confidence: ConfidenceTrustedChecksum,
	}
	right := Extent{
		StartLBA:   104,
		Sectors:    2,
		State:      SectorStateVerified,
		Confidence: ConfidenceTrustedChecksum,
	}

	merged, err := MergeAdjacent(left, right)
	if err != nil {
		t.Fatalf("unexpected merge error: %v", err)
	}
	if merged.StartLBA != 100 || merged.Sectors != 6 {
		t.Fatalf("unexpected merged extent: %+v", merged)
	}
}

func TestLookupExtent(t *testing.T) {
	extents := []Extent{
		{StartLBA: 0, Sectors: 4, State: SectorStateUnknown, Confidence: ConfidenceNone},
		{StartLBA: 4, Sectors: 2, State: SectorStateMissing, Confidence: ConfidenceNone},
	}

	extent, index, ok := LookupExtent(extents, 5)
	if !ok {
		t.Fatal("expected extent lookup to succeed")
	}
	if index != 1 || extent.StartLBA != 4 {
		t.Fatalf("unexpected lookup result: index=%d extent=%+v", index, extent)
	}
}

func TestInsertExtentRejectsOverlap(t *testing.T) {
	extents := []Extent{
		{StartLBA: 0, Sectors: 4, State: SectorStateUnknown, Confidence: ConfidenceNone},
	}

	_, err := InsertExtent(extents, Extent{StartLBA: 3, Sectors: 2, State: SectorStateMissing, Confidence: ConfidenceNone})
	if err == nil {
		t.Fatal("expected overlapping insert to fail")
	}
}

func TestInsertExtentCoalescesCompatibleNeighbors(t *testing.T) {
	extents := []Extent{
		{StartLBA: 0, Sectors: 4, State: SectorStateVerified, Confidence: ConfidenceRepeatedSingleCapture},
		{StartLBA: 8, Sectors: 4, State: SectorStateVerified, Confidence: ConfidenceRepeatedSingleCapture},
	}

	result, err := InsertExtent(extents, Extent{StartLBA: 4, Sectors: 4, State: SectorStateVerified, Confidence: ConfidenceRepeatedSingleCapture})
	if err != nil {
		t.Fatalf("unexpected insert error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected one coalesced extent, got %d", len(result))
	}
	if result[0].StartLBA != 0 || result[0].Sectors != 12 {
		t.Fatalf("unexpected coalesced extent: %+v", result[0])
	}
}

func TestApplyExtentRefinesDeferredSubrange(t *testing.T) {
	extents := []Extent{
		{StartLBA: 64, Sectors: 64, State: SectorStateUnknown, Confidence: ConfidenceNone, Attempts: 1},
	}
	candidate := Extent{
		StartLBA:   80,
		Sectors:    16,
		State:      SectorStateReadUnverified,
		Confidence: ConfidenceSingleRead,
		Attempts:   2,
	}

	result, err := ApplyExtent(extents, candidate)
	if err != nil {
		t.Fatalf("apply extent: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected deferred tails around recovered range, got %+v", result)
	}
	if result[0].StartLBA != 64 || result[0].Sectors != 16 || result[0].State != SectorStateUnknown {
		t.Fatalf("unexpected left deferred tail: %+v", result[0])
	}
	if result[1].StartLBA != 80 || result[1].Sectors != 16 || result[1].State != SectorStateReadUnverified {
		t.Fatalf("unexpected recovered middle: %+v", result[1])
	}
	if result[2].StartLBA != 96 || result[2].Sectors != 32 || result[2].State != SectorStateUnknown {
		t.Fatalf("unexpected right deferred tail: %+v", result[2])
	}
}

func TestApplyExtentRejectsRecoveredDataReplacement(t *testing.T) {
	extents := []Extent{
		{StartLBA: 10, Sectors: 4, State: SectorStateReadUnverified, Confidence: ConfidenceSingleRead, Attempts: 1},
	}
	candidate := Extent{
		StartLBA:   10,
		Sectors:    4,
		State:      SectorStateMissing,
		Confidence: ConfidenceNone,
		Attempts:   2,
	}

	if _, err := ApplyExtent(extents, candidate); err == nil {
		t.Fatal("expected recovered data to reject transition to missing")
	}
}

func TestApplyExtentRejectsStaleAttempt(t *testing.T) {
	extents := []Extent{
		{StartLBA: 20, Sectors: 4, State: SectorStateUnknown, Confidence: ConfidenceNone, Attempts: 3},
	}
	candidate := Extent{
		StartLBA:   20,
		Sectors:    4,
		State:      SectorStateQueued,
		Confidence: ConfidenceNone,
		Attempts:   2,
	}

	if _, err := ApplyExtent(extents, candidate); err == nil {
		t.Fatal("expected stale attempt to be rejected")
	}
}
