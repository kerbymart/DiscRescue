package mapfile

import "testing"

func TestExtentValidateRejectsZeroSectors(t *testing.T) {
	err := (Extent{StartLBA: 10, Sectors: 0, State: SectorStateUnknown, Confidence: ConfidenceNone}).Validate()
	if err == nil {
		t.Fatal("expected zero-sector extent to fail validation")
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

func TestApplyExtentReplacesOverlappingRange(t *testing.T) {
	extents := []Extent{
		{StartLBA: 0, Sectors: 8, State: SectorStateSkipped, Confidence: ConfidenceNone},
	}

	result, err := ApplyExtent(extents, Extent{
		StartLBA:   2,
		Sectors:    3,
		State:      SectorStateReadUnverified,
		Confidence: ConfidenceSingleRead,
	})
	if err != nil {
		t.Fatalf("apply extent: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected split result, got %+v", result)
	}
	if result[0].StartLBA != 0 || result[0].Sectors != 2 || result[0].State != SectorStateSkipped {
		t.Fatalf("unexpected left extent: %+v", result[0])
	}
	if result[1].StartLBA != 2 || result[1].Sectors != 3 || result[1].State != SectorStateReadUnverified {
		t.Fatalf("unexpected middle extent: %+v", result[1])
	}
	if result[2].StartLBA != 5 || result[2].Sectors != 3 || result[2].State != SectorStateSkipped {
		t.Fatalf("unexpected right extent: %+v", result[2])
	}
}
