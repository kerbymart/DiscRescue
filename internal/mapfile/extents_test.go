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
