package testdevice

import "testing"

func TestScenarioValidateAcceptsDeterministicMedia(t *testing.T) {
	scenario := Scenario{
		Name: "healthy-data-media",
		Media: Media{
			MediaID:           "disc-001",
			Profile:           0x0010,
			LogicalSectorSize: 2048,
			SectorCount:       100,
			Sessions:          1,
			Tracks:            1,
			Data: []DataSpec{
				{
					Range:   Range{StartLBA: 0, Sectors: 100},
					Pattern: PatternLBA,
				},
			},
		},
		Delays: []DelaySpec{
			{
				Range:    Range{StartLBA: 10, Sectors: 4},
				Millis:   25,
				Selector: AttemptSelector{Pass: PassFast, AttemptMin: 1, AttemptMax: 2},
			},
		},
		Failures: []FailureSpec{
			{
				Range:    Range{StartLBA: 40, Sectors: 1},
				Mode:     FailureReadError,
				Selector: AttemptSelector{Pass: PassTrim, AttemptMin: 1},
				Sense:    SenseTuple{Key: 0x03, ASC: 0x11, ASCQ: 0x00},
			},
		},
	}

	if err := scenario.Validate(); err != nil {
		t.Fatalf("expected scenario to be valid, got error: %v", err)
	}
}

func TestScenarioValidateRejectsOverlappingDataRanges(t *testing.T) {
	scenario := Scenario{
		Name: "overlap",
		Media: Media{
			MediaID:           "disc-001",
			LogicalSectorSize: 2048,
			SectorCount:       10,
			Data: []DataSpec{
				{Range: Range{StartLBA: 0, Sectors: 5}, Pattern: PatternZero},
				{Range: Range{StartLBA: 4, Sectors: 2}, Pattern: PatternZero},
			},
		},
	}

	if err := scenario.Validate(); err == nil {
		t.Fatal("expected overlapping data ranges to fail validation")
	}
}

func TestFailureSpecValidateRejectsMissingReplacementMediaID(t *testing.T) {
	spec := FailureSpec{
		Range:    Range{StartLBA: 12, Sectors: 1},
		Mode:     FailureMediaReplacement,
		Selector: AttemptSelector{Pass: PassAny, AttemptMin: 1},
	}

	if err := spec.Validate(); err == nil {
		t.Fatal("expected replacement failure without media id to fail validation")
	}
}

func TestDataSpecValidateRejectsWrongSectorPatternLength(t *testing.T) {
	spec := DataSpec{
		Range:   Range{StartLBA: 0, Sectors: 1},
		Pattern: PatternBytes,
		Bytes:   []byte{1, 2, 3},
	}

	if err := spec.Validate(2048); err == nil {
		t.Fatal("expected invalid byte pattern length to fail validation")
	}
}
