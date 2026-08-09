package catalog

import "fmt"

type PriorProcessingState string

const (
	PriorProcessingNone          PriorProcessingState = "none"
	PriorProcessingSeen          PriorProcessingState = "seen"
	PriorProcessingProcessed     PriorProcessingState = "processed"
	PriorProcessingArchived      PriorProcessingState = "archived"
	PriorProcessingIncomplete    PriorProcessingState = "incomplete"
	PriorProcessingUnavailable   PriorProcessingState = "unavailable"
	PriorProcessingConflict      PriorProcessingState = "conflict"
	PriorProcessingPossibleMatch PriorProcessingState = "possible_match"
	PriorProcessingIndeterminate PriorProcessingState = "indeterminate"
)

type PriorProcessingCopy struct {
	State      PriorProcessingState
	Title      string
	Detail     string
	Candidates []Entry
}

func BuildPriorProcessingCopy(result LookupResult) (PriorProcessingCopy, error) {
	for i, candidate := range result.Candidates {
		if err := candidate.Validate(); err != nil {
			return PriorProcessingCopy{}, fmt.Errorf("build prior-processing copy candidate[%d]: %w", i, err)
		}
	}

	switch result.Match {
	case MatchNo:
		return PriorProcessingCopy{
			State:  PriorProcessingNone,
			Detail: "History: no matching contents found on this computer",
		}, nil
	case MatchIndeterminate:
		return PriorProcessingCopy{
			State:  PriorProcessingIndeterminate,
			Detail: "History: could not identify enough readable areas to check reliably",
		}, nil
	case MatchProbable:
		return PriorProcessingCopy{
			State:      PriorProcessingPossibleMatch,
			Title:      "These contents may have been processed before",
			Detail:     "The disc layout and readable samples match a previous job, but there was not enough readable data for a strong match.",
			Candidates: append([]Entry(nil), result.Candidates...),
		}, nil
	case MatchConflict:
		return PriorProcessingCopy{
			State:      PriorProcessingConflict,
			Title:      "Matching contents conflict with a previous record",
			Detail:     "Automatic resume and merge are blocked until the conflict is reviewed.",
			Candidates: append([]Entry(nil), result.Candidates...),
		}, nil
	case MatchStrong:
		if len(result.Candidates) == 0 {
			return PriorProcessingCopy{}, fmt.Errorf("build prior-processing copy: strong match requires a candidate")
		}
		return strongPriorProcessingCopy(result.Candidates[0]), nil
	default:
		return PriorProcessingCopy{}, fmt.Errorf("build prior-processing copy: unsupported match %q", result.Match)
	}
}

func strongPriorProcessingCopy(entry Entry) PriorProcessingCopy {
	if !entryFilesPresent(entry) {
		return PriorProcessingCopy{
			State:      PriorProcessingUnavailable,
			Title:      "Matching contents were processed before",
			Detail:     "Previous files not found",
			Candidates: []Entry{entry},
		}
	}

	switch entry.State {
	case ProcessingObserved:
		return PriorProcessingCopy{
			State:      PriorProcessingSeen,
			Title:      "Matching contents were seen before",
			Detail:     "Seen before; no recovery was started",
			Candidates: []Entry{entry},
		}
	case ProcessingInProgress:
		return PriorProcessingCopy{
			State:      PriorProcessingProcessed,
			Title:      "Recovery was started before",
			Detail:     "Recovery may still be active",
			Candidates: []Entry{entry},
		}
	case ProcessingStoppedResumable:
		return PriorProcessingCopy{
			State:      PriorProcessingIncomplete,
			Title:      "Recovery was started before",
			Detail:     "Recovery incomplete; progress can be resumed",
			Candidates: []Entry{entry},
		}
	case ProcessingCompletedVerified:
		return PriorProcessingCopy{
			State:      PriorProcessingArchived,
			Title:      "Matching contents were processed before",
			Detail:     "Archived successfully",
			Candidates: []Entry{entry},
		}
	case ProcessingCompleted:
		return PriorProcessingCopy{
			State:      PriorProcessingArchived,
			Title:      "Matching contents were processed before",
			Detail:     "Completed successfully",
			Candidates: []Entry{entry},
		}
	case ProcessingCompletedWithGaps:
		return PriorProcessingCopy{
			State:      PriorProcessingArchived,
			Title:      "Matching contents were processed before",
			Detail:     "Archived with unreadable sectors",
			Candidates: []Entry{entry},
		}
	case ProcessingFailed:
		return PriorProcessingCopy{
			State:      PriorProcessingProcessed,
			Title:      "Recovery was started before",
			Detail:     "Previous attempt failed",
			Candidates: []Entry{entry},
		}
	case ProcessingMerged:
		return PriorProcessingCopy{
			State:      PriorProcessingArchived,
			Title:      "Matching contents were processed before",
			Detail:     "Archive produced from multiple captures",
			Candidates: []Entry{entry},
		}
	default:
		return PriorProcessingCopy{
			State:      PriorProcessingProcessed,
			Title:      "Matching contents were processed before",
			Detail:     string(entry.State),
			Candidates: []Entry{entry},
		}
	}
}

func entryFilesPresent(entry Entry) bool {
	if len(entry.JobReferences) == 0 {
		return true
	}
	if entry.PreferredJobID != (RecordID{}) {
		for _, reference := range entry.JobReferences {
			if reference.JobID == entry.PreferredJobID {
				return reference.FilesPresent
			}
		}
	}
	for _, reference := range entry.JobReferences {
		if reference.FilesPresent {
			return true
		}
	}
	return false
}
