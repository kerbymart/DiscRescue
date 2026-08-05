package image

import (
	"fmt"

	"discrescue/internal/mapfile"
)

type CommitOutcome string

const (
	CommitOutcomeSuccess CommitOutcome = "success"
	CommitOutcomeENOSPC  CommitOutcome = "enospc"
)

type ResumableState struct {
	LastDurableExtent     *mapfile.Extent
	SchedulingStopped     bool
	NeedsUserIntervention bool
	Resumable             bool
	UnrecoverableMetadata bool
	StatusMessage         string
}

type ENOSPCEvaluation struct {
	CommitSequence      CommitSequence
	FailedStage         CommitStage
	PreviousDurablePair *mapfile.Extent
	PendingExtent       mapfile.Extent
}

func EvaluateENOSPC(input ENOSPCEvaluation) (ResumableState, error) {
	if err := input.PendingExtent.Validate(); err != nil {
		return ResumableState{}, fmt.Errorf("evaluate enospc: pending extent: %w", err)
	}
	if input.PreviousDurablePair != nil {
		if err := input.PreviousDurablePair.Validate(); err != nil {
			return ResumableState{}, fmt.Errorf("evaluate enospc: previous durable extent: %w", err)
		}
	}
	if !input.CommitSequence.Contains(input.FailedStage) {
		return ResumableState{}, fmt.Errorf("evaluate enospc: failed stage %s is not present in commit sequence", input.FailedStage)
	}

	state := ResumableState{
		LastDurableExtent:     input.PreviousDurablePair,
		SchedulingStopped:     true,
		NeedsUserIntervention: true,
		Resumable:             true,
		StatusMessage:         "Output storage is full. Recovery stopped before advancing durable map state.",
	}

	switch input.FailedStage {
	case StageWriteImage, StageSyncImage:
		return state, nil
	case StageAppendJournal, StageSyncJournal:
		state.StatusMessage = "Output storage is full after image durability, but before journal completion. Resume from the last durable map state."
		return state, nil
	default:
		return ResumableState{}, fmt.Errorf("evaluate enospc: stage %s is not an ENOSPC stage", input.FailedStage)
	}
}
