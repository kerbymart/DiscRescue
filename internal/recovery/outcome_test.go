package recovery

import (
	"testing"

	"discrescue/internal/mapfile"
)

func TestEvaluateOutcomeCompleteVerified(t *testing.T) {
	result, err := EvaluateOutcome(OutcomeInput{
		Extents: []mapfile.Extent{
			{StartLBA: 0, Sectors: 4, State: mapfile.SectorStateVerified, Confidence: mapfile.ConfidenceTrustedChecksum},
			{StartLBA: 4, Sectors: 2, State: mapfile.SectorStateReconstructed, Confidence: mapfile.ConfidenceReconstructedChecksum},
		},
		ValidImageMapPair: true,
	})
	if err != nil {
		t.Fatalf("evaluate outcome: %v", err)
	}
	if result.Class != OutcomeCompleteVerified {
		t.Fatalf("unexpected outcome class: %s", result.Class)
	}
	if !result.FullyVerified || result.Resumable {
		t.Fatalf("unexpected flags: %+v", result)
	}
}

func TestEvaluateOutcomeCompleteUnverifiedWhenPolicyAllowsIt(t *testing.T) {
	result, err := EvaluateOutcome(OutcomeInput{
		Extents: []mapfile.Extent{
			{StartLBA: 0, Sectors: 4, State: mapfile.SectorStateVerified, Confidence: mapfile.ConfidenceTrustedChecksum},
			{StartLBA: 4, Sectors: 2, State: mapfile.SectorStateReadUnverified, Confidence: mapfile.ConfidenceSingleRead},
		},
		AcceptUnverifiedByPolicy: true,
		ValidImageMapPair:        true,
	})
	if err != nil {
		t.Fatalf("evaluate outcome: %v", err)
	}
	if result.Class != OutcomeCompleteUnverified {
		t.Fatalf("unexpected outcome class: %s", result.Class)
	}
	if result.FullyVerified || result.AcceptedUnverifiedSectors != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestEvaluateOutcomeIncompleteAfterPassesExhausted(t *testing.T) {
	result, err := EvaluateOutcome(OutcomeInput{
		Extents: []mapfile.Extent{
			{StartLBA: 0, Sectors: 4, State: mapfile.SectorStateVerified, Confidence: mapfile.ConfidenceTrustedChecksum},
			{StartLBA: 4, Sectors: 2, State: mapfile.SectorStateMissing, Confidence: mapfile.ConfidenceNone},
		},
		PassesExhausted:   true,
		ValidImageMapPair: true,
	})
	if err != nil {
		t.Fatalf("evaluate outcome: %v", err)
	}
	if result.Class != OutcomeIncomplete {
		t.Fatalf("unexpected outcome class: %s", result.Class)
	}
	if !result.Resumable || result.UnresolvedSectors != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestEvaluateOutcomeStoppedResumable(t *testing.T) {
	result, err := EvaluateOutcome(OutcomeInput{
		Extents: []mapfile.Extent{
			{StartLBA: 0, Sectors: 4, State: mapfile.SectorStateVerified, Confidence: mapfile.ConfidenceTrustedChecksum},
			{StartLBA: 4, Sectors: 2, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
		},
		SchedulingStopped: true,
		ValidImageMapPair: true,
	})
	if err != nil {
		t.Fatalf("evaluate outcome: %v", err)
	}
	if result.Class != OutcomeStoppedResumable {
		t.Fatalf("unexpected outcome class: %s", result.Class)
	}
	if !result.Resumable {
		t.Fatalf("expected resumable result, got %+v", result)
	}
}

func TestEvaluateOutcomeFailedWithoutValidPair(t *testing.T) {
	result, err := EvaluateOutcome(OutcomeInput{
		Extents: []mapfile.Extent{
			{StartLBA: 0, Sectors: 4, State: mapfile.SectorStateUnknown, Confidence: mapfile.ConfidenceNone},
		},
		ValidImageMapPair: false,
	})
	if err != nil {
		t.Fatalf("evaluate outcome: %v", err)
	}
	if result.Class != OutcomeFailedNoValidPair {
		t.Fatalf("unexpected outcome class: %s", result.Class)
	}
	if result.Resumable {
		t.Fatalf("did not expect resumable failed result: %+v", result)
	}
}

func TestEvaluateOutcomeInProgressWhenWorkRemainsAndPassesNotExhausted(t *testing.T) {
	result, err := EvaluateOutcome(OutcomeInput{
		Extents: []mapfile.Extent{
			{StartLBA: 0, Sectors: 4, State: mapfile.SectorStateVerified, Confidence: mapfile.ConfidenceTrustedChecksum},
			{StartLBA: 4, Sectors: 2, State: mapfile.SectorStateIOError, Confidence: mapfile.ConfidenceNone},
		},
		ValidImageMapPair: true,
	})
	if err != nil {
		t.Fatalf("evaluate outcome: %v", err)
	}
	if result.Class != OutcomeInProgress {
		t.Fatalf("unexpected outcome class: %s", result.Class)
	}
	if !result.Resumable {
		t.Fatalf("expected in-progress result to remain resumable, got %+v", result)
	}
}

func TestEvaluateOutcomeRejectsInvalidExtentSet(t *testing.T) {
	_, err := EvaluateOutcome(OutcomeInput{
		Extents: []mapfile.Extent{
			{StartLBA: 4, Sectors: 2, State: mapfile.SectorStateVerified, Confidence: mapfile.ConfidenceTrustedChecksum},
			{StartLBA: 0, Sectors: 4, State: mapfile.SectorStateVerified, Confidence: mapfile.ConfidenceTrustedChecksum},
		},
		ValidImageMapPair: true,
	})
	if err == nil {
		t.Fatal("expected invalid extent set to be rejected")
	}
}
