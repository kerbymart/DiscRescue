package image

import (
	"fmt"

	"discrescue/internal/mapfile"
)

type DurabilityProfile string

const (
	DurabilityFast     DurabilityProfile = "fast"
	DurabilityBalanced DurabilityProfile = "balanced"
	DurabilityStrict   DurabilityProfile = "strict"
)

type CommitStage string

const (
	StageValidateBounds CommitStage = "validate_bounds"
	StageWriteImage     CommitStage = "write_image"
	StageSyncImage      CommitStage = "sync_image"
	StageAppendJournal  CommitStage = "append_journal"
	StageSyncJournal    CommitStage = "sync_journal"
	StagePublishUI      CommitStage = "publish_ui"
)

type RecoveredSectorCommit struct {
	LogicalSectorSize uint32
	ExpectedSectors   uint64
	Write             SectorWrite
	Extent            mapfile.Extent
	Profile           DurabilityProfile
}

type FailedSectorCommit struct {
	Extent mapfile.Extent
}

type CommitAction struct {
	Stage CommitStage
}

type CommitSequence struct {
	Actions []CommitAction
}

func BuildRecoveredSectorCommit(input RecoveredSectorCommit) (CommitSequence, error) {
	if input.Profile == "" {
		return CommitSequence{}, fmt.Errorf("build recovered sector commit: durability profile is required")
	}
	if input.Write.LogicalSectorSize != input.LogicalSectorSize {
		return CommitSequence{}, fmt.Errorf("build recovered sector commit: write sector size %d does not match commit size %d", input.Write.LogicalSectorSize, input.LogicalSectorSize)
	}
	if input.Extent.State != mapfile.SectorStateReadUnverified &&
		input.Extent.State != mapfile.SectorStateVerified &&
		input.Extent.State != mapfile.SectorStateReconstructed {
		return CommitSequence{}, fmt.Errorf("build recovered sector commit: extent state %s is not a recovered-data state", input.Extent.State)
	}
	if err := input.Extent.Validate(); err != nil {
		return CommitSequence{}, fmt.Errorf("build recovered sector commit: %w", err)
	}
	if _, err := BuildPositionedWrites(WriterPlan{
		LogicalSectorSize: input.LogicalSectorSize,
		ExpectedSectors:   input.ExpectedSectors,
		Writes:            []SectorWrite{input.Write},
	}); err != nil {
		return CommitSequence{}, fmt.Errorf("build recovered sector commit: %w", err)
	}
	if input.Write.LBA != input.Extent.StartLBA || input.Extent.Sectors != 1 {
		return CommitSequence{}, fmt.Errorf("build recovered sector commit: write and extent must describe the same single sector")
	}

	actions := []CommitAction{
		{Stage: StageValidateBounds},
		{Stage: StageWriteImage},
	}
	if input.Profile == DurabilityStrict {
		actions = append(actions, CommitAction{Stage: StageSyncImage})
	}
	actions = append(actions,
		CommitAction{Stage: StageAppendJournal},
		CommitAction{Stage: StageSyncJournal},
		CommitAction{Stage: StagePublishUI},
	)

	return CommitSequence{Actions: actions}, nil
}

func BuildFailedSectorCommit(input FailedSectorCommit) (CommitSequence, error) {
	if input.Extent.State != mapfile.SectorStateMissing &&
		input.Extent.State != mapfile.SectorStateIOError &&
		input.Extent.State != mapfile.SectorStateSkipped &&
		input.Extent.State != mapfile.SectorStateChecksumError &&
		input.Extent.State != mapfile.SectorStateConflicting {
		return CommitSequence{}, fmt.Errorf("build failed sector commit: extent state %s is not a failed-data state", input.Extent.State)
	}
	if err := input.Extent.Validate(); err != nil {
		return CommitSequence{}, fmt.Errorf("build failed sector commit: %w", err)
	}

	return CommitSequence{
		Actions: []CommitAction{
			{Stage: StageAppendJournal},
			{Stage: StageSyncJournal},
			{Stage: StagePublishUI},
		},
	}, nil
}

func SimulateCrash(sequence CommitSequence, stage CommitStage) CommitSequence {
	result := CommitSequence{}
	for _, action := range sequence.Actions {
		if action.Stage == stage {
			break
		}
		result.Actions = append(result.Actions, action)
	}
	return result
}

func (s CommitSequence) Contains(stage CommitStage) bool {
	for _, action := range s.Actions {
		if action.Stage == stage {
			return true
		}
	}
	return false
}
