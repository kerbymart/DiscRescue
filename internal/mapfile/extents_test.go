package mapfile

import "testing"

func TestExtentValidateRejectsZeroSectors(t *testing.T) {
	err := (Extent{StartLBA: 10, Sectors: 0, State: SectorStateUnknown}).Validate()
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
