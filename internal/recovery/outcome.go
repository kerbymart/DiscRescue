package recovery

import (
	"fmt"

	"discrescue/internal/mapfile"
)

type OutcomeClass string

const (
	OutcomeInProgress         OutcomeClass = "in_progress"
	OutcomeCompleteVerified   OutcomeClass = "complete_verified"
	OutcomeCompleteUnverified OutcomeClass = "complete_unverified"
	OutcomeIncomplete         OutcomeClass = "incomplete"
	OutcomeStoppedResumable   OutcomeClass = "stopped_resumable"
	OutcomeFailedNoValidPair  OutcomeClass = "failed_no_valid_pair"
)

type OutcomeInput struct {
	Extents                  []mapfile.Extent
	AcceptUnverifiedByPolicy bool
	PassesExhausted          bool
	SchedulingStopped        bool
	ValidImageMapPair        bool
}

type OutcomeResult struct {
	Class                     OutcomeClass
	PlainStatus               string
	Resumable                 bool
	FullyVerified             bool
	VerifiedSectors           uint64
	ReconstructedSectors      uint64
	AcceptedUnverifiedSectors uint64
	UnresolvedSectors         uint64
}

func EvaluateOutcome(input OutcomeInput) (OutcomeResult, error) {
	if err := mapfile.ValidateExtentSet(input.Extents); err != nil {
		return OutcomeResult{}, err
	}
	if !input.ValidImageMapPair {
		return OutcomeResult{
			Class:         OutcomeFailedNoValidPair,
			PlainStatus:   "Recovery failed before a valid image/map pair was established.",
			Resumable:     false,
			FullyVerified: false,
		}, nil
	}

	var result OutcomeResult
	for _, extent := range input.Extents {
		sectors := uint64(extent.Sectors)
		switch extent.State {
		case mapfile.SectorStateVerified:
			result.VerifiedSectors += sectors
		case mapfile.SectorStateReconstructed:
			result.ReconstructedSectors += sectors
		case mapfile.SectorStateReadUnverified:
			result.AcceptedUnverifiedSectors += sectors
		case mapfile.SectorStateUnknown,
			mapfile.SectorStateQueued,
			mapfile.SectorStateMissing,
			mapfile.SectorStateIOError,
			mapfile.SectorStateChecksumError,
			mapfile.SectorStateConflicting,
			mapfile.SectorStateSkipped:
			result.UnresolvedSectors += sectors
		default:
			return OutcomeResult{}, fmt.Errorf("evaluate outcome: unsupported sector state %s", extent.State)
		}
	}

	hasUnverified := result.AcceptedUnverifiedSectors > 0
	if result.UnresolvedSectors == 0 && (!hasUnverified || input.AcceptUnverifiedByPolicy) {
		if hasUnverified {
			result.Class = OutcomeCompleteUnverified
			result.PlainStatus = "Image created with unverified sectors."
			result.Resumable = false
			result.FullyVerified = false
			return result, nil
		}
		result.Class = OutcomeCompleteVerified
		result.PlainStatus = "Image created successfully and fully verified."
		result.Resumable = false
		result.FullyVerified = true
		return result, nil
	}

	if input.SchedulingStopped {
		result.Class = OutcomeStoppedResumable
		result.PlainStatus = "Recovery stopped but resumable."
		result.Resumable = true
		result.FullyVerified = false
		return result, nil
	}

	if input.PassesExhausted {
		result.Class = OutcomeIncomplete
		result.PlainStatus = "Image created with missing sectors."
		result.Resumable = true
		result.FullyVerified = false
		return result, nil
	}

	result.Class = OutcomeInProgress
	result.PlainStatus = "Recovery is still in progress."
	result.Resumable = true
	result.FullyVerified = false
	return result, nil
}
