package image

import (
	"testing"

	"discrescue/internal/mapfile"
)

func TestEvaluateENOSPCBeforeJournalPreservesPreviousDurablePair(t *testing.T) {
	previous := mapfile.Extent{
		StartLBA:   10,
		Sectors:    1,
		State:      mapfile.SectorStateVerified,
		Confidence: mapfile.ConfidenceTrustedChecksum,
	}
	pending := mapfile.Extent{
		StartLBA:   11,
		Sectors:    1,
		State:      mapfile.SectorStateReadUnverified,
		Confidence: mapfile.ConfidenceSingleRead,
	}
	sequence, err := BuildRecoveredSectorCommit(RecoveredSectorCommit{
		LogicalSectorSize: 2048,
		ExpectedSectors:   32,
		Write: SectorWrite{
			LBA:               11,
			LogicalSectorSize: 2048,
			Data:              make([]byte, 2048),
		},
		Extent:  pending,
		Profile: DurabilityBalanced,
	})
	if err != nil {
		t.Fatalf("build recovered sector commit: %v", err)
	}

	state, err := EvaluateENOSPC(ENOSPCEvaluation{
		CommitSequence:      sequence,
		FailedStage:         StageWriteImage,
		PreviousDurablePair: &previous,
		PendingExtent:       pending,
	})
	if err != nil {
		t.Fatalf("evaluate enospc: %v", err)
	}
	if !state.SchedulingStopped || !state.Resumable || !state.NeedsUserIntervention {
		t.Fatalf("unexpected resumable state flags: %+v", state)
	}
	if state.LastDurableExtent == nil || state.LastDurableExtent.StartLBA != previous.StartLBA {
		t.Fatalf("expected previous durable extent to be preserved, got %+v", state.LastDurableExtent)
	}
}

func TestEvaluateENOSPCAfterImageBeforeJournalKeepsResumePossible(t *testing.T) {
	pending := mapfile.Extent{
		StartLBA:   4,
		Sectors:    1,
		State:      mapfile.SectorStateVerified,
		Confidence: mapfile.ConfidenceRepeatedSingleCapture,
	}
	sequence, err := BuildRecoveredSectorCommit(RecoveredSectorCommit{
		LogicalSectorSize: 2048,
		ExpectedSectors:   32,
		Write: SectorWrite{
			LBA:               4,
			LogicalSectorSize: 2048,
			Data:              make([]byte, 2048),
		},
		Extent:  pending,
		Profile: DurabilityStrict,
	})
	if err != nil {
		t.Fatalf("build recovered sector commit: %v", err)
	}

	state, err := EvaluateENOSPC(ENOSPCEvaluation{
		CommitSequence: sequence,
		FailedStage:    StageAppendJournal,
		PendingExtent:  pending,
	})
	if err != nil {
		t.Fatalf("evaluate enospc: %v", err)
	}
	if !state.Resumable || !state.SchedulingStopped {
		t.Fatalf("expected resumable stopped state, got %+v", state)
	}
	if state.UnrecoverableMetadata {
		t.Fatalf("expected metadata to remain recoverable, got %+v", state)
	}
}

func TestEvaluateENOSPCRejectsUnknownStage(t *testing.T) {
	pending := mapfile.Extent{
		StartLBA:   2,
		Sectors:    1,
		State:      mapfile.SectorStateMissing,
		Confidence: mapfile.ConfidenceNone,
	}
	sequence, err := BuildFailedSectorCommit(FailedSectorCommit{Extent: pending})
	if err != nil {
		t.Fatalf("build failed sector commit: %v", err)
	}

	if _, err := EvaluateENOSPC(ENOSPCEvaluation{
		CommitSequence: sequence,
		FailedStage:    StagePublishUI,
		PendingExtent:  pending,
	}); err == nil {
		t.Fatal("expected unsupported enospc stage to fail")
	}
}

func TestEvaluateENOSPCRejectsStageOutsideSequence(t *testing.T) {
	pending := mapfile.Extent{
		StartLBA:   7,
		Sectors:    1,
		State:      mapfile.SectorStateReadUnverified,
		Confidence: mapfile.ConfidenceSingleRead,
	}
	sequence, err := BuildRecoveredSectorCommit(RecoveredSectorCommit{
		LogicalSectorSize: 2048,
		ExpectedSectors:   32,
		Write: SectorWrite{
			LBA:               7,
			LogicalSectorSize: 2048,
			Data:              make([]byte, 2048),
		},
		Extent:  pending,
		Profile: DurabilityBalanced,
	})
	if err != nil {
		t.Fatalf("build recovered sector commit: %v", err)
	}

	if _, err := EvaluateENOSPC(ENOSPCEvaluation{
		CommitSequence: sequence,
		FailedStage:    StageSyncImage,
		PendingExtent:  pending,
	}); err == nil {
		t.Fatal("expected missing stage in commit sequence to fail")
	}
}
