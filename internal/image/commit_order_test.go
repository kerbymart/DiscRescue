package image

import (
	"testing"

	"discrescue/internal/mapfile"
)

func TestBuildRecoveredSectorCommitOrdersImageBeforeMap(t *testing.T) {
	sequence, err := BuildRecoveredSectorCommit(RecoveredSectorCommit{
		LogicalSectorSize: 2048,
		ExpectedSectors:   16,
		Write: SectorWrite{
			LBA:               3,
			LogicalSectorSize: 2048,
			Data:              make([]byte, 2048),
		},
		Extent: mapfile.Extent{
			StartLBA:   3,
			Sectors:    1,
			State:      mapfile.SectorStateReadUnverified,
			Confidence: mapfile.ConfidenceSingleRead,
		},
		Profile: DurabilityBalanced,
	})
	if err != nil {
		t.Fatalf("build recovered commit: %v", err)
	}

	want := []CommitStage{
		StageValidateBounds,
		StageWriteImage,
		StageAppendJournal,
		StageSyncJournal,
		StagePublishUI,
	}
	if len(sequence.Actions) != len(want) {
		t.Fatalf("unexpected action count: got %d want %d", len(sequence.Actions), len(want))
	}
	for i, stage := range want {
		if sequence.Actions[i].Stage != stage {
			t.Fatalf("unexpected stage at %d: got %s want %s", i, sequence.Actions[i].Stage, stage)
		}
	}
}

func TestBuildRecoveredSectorCommitStrictAddsImageSync(t *testing.T) {
	sequence, err := BuildRecoveredSectorCommit(RecoveredSectorCommit{
		LogicalSectorSize: 2048,
		ExpectedSectors:   16,
		Write: SectorWrite{
			LBA:               0,
			LogicalSectorSize: 2048,
			Data:              make([]byte, 2048),
		},
		Extent: mapfile.Extent{
			StartLBA:   0,
			Sectors:    1,
			State:      mapfile.SectorStateVerified,
			Confidence: mapfile.ConfidenceTrustedChecksum,
		},
		Profile: DurabilityStrict,
	})
	if err != nil {
		t.Fatalf("build recovered commit: %v", err)
	}
	if !sequence.Contains(StageSyncImage) {
		t.Fatal("expected strict durability profile to require image sync")
	}
}

func TestBuildRecoveredSectorCommitRejectsJournalBeforeImageCrashPoint(t *testing.T) {
	sequence, err := BuildRecoveredSectorCommit(RecoveredSectorCommit{
		LogicalSectorSize: 2048,
		ExpectedSectors:   16,
		Write: SectorWrite{
			LBA:               5,
			LogicalSectorSize: 2048,
			Data:              make([]byte, 2048),
		},
		Extent: mapfile.Extent{
			StartLBA:   5,
			Sectors:    1,
			State:      mapfile.SectorStateReadUnverified,
			Confidence: mapfile.ConfidenceSingleRead,
		},
		Profile: DurabilityBalanced,
	})
	if err != nil {
		t.Fatalf("build recovered commit: %v", err)
	}

	crashed := SimulateCrash(sequence, StageAppendJournal)
	if crashed.Contains(StageAppendJournal) {
		t.Fatal("expected crash before journal append to prevent durable map transition")
	}
	if !crashed.Contains(StageWriteImage) {
		t.Fatal("expected crash before journal append to preserve prior image write step")
	}
}

func TestBuildFailedSectorCommitNeverSchedulesImageWrite(t *testing.T) {
	sequence, err := BuildFailedSectorCommit(FailedSectorCommit{
		Extent: mapfile.Extent{
			StartLBA:   8,
			Sectors:    1,
			State:      mapfile.SectorStateMissing,
			Confidence: mapfile.ConfidenceNone,
		},
	})
	if err != nil {
		t.Fatalf("build failed commit: %v", err)
	}
	if sequence.Contains(StageWriteImage) {
		t.Fatal("failed sector commit must not write image data")
	}
}

func TestBuildRecoveredSectorCommitRejectsMismatchedExtentAndWrite(t *testing.T) {
	_, err := BuildRecoveredSectorCommit(RecoveredSectorCommit{
		LogicalSectorSize: 2048,
		ExpectedSectors:   16,
		Write: SectorWrite{
			LBA:               1,
			LogicalSectorSize: 2048,
			Data:              make([]byte, 2048),
		},
		Extent: mapfile.Extent{
			StartLBA:   2,
			Sectors:    1,
			State:      mapfile.SectorStateReadUnverified,
			Confidence: mapfile.ConfidenceSingleRead,
		},
		Profile: DurabilityBalanced,
	})
	if err == nil {
		t.Fatal("expected mismatched extent and write to fail")
	}
}
