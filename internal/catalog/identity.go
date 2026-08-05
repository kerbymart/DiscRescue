package catalog

type MatchStrength string

const (
	MatchStrong        MatchStrength = "strong"
	MatchProbable      MatchStrength = "probable"
	MatchIndeterminate MatchStrength = "indeterminate"
	MatchConflict      MatchStrength = "conflict"
	MatchNo            MatchStrength = "none"
)

type ContentIdentity struct {
	Version           uint16
	Profile           uint16
	LogicalBlockSize  uint32
	SectorCount       uint64
	LayoutSHA256      string
	QuickID           string
	FullContentSHA256 string
}

type ProcessingState string

const (
	ProcessingObserved          ProcessingState = "observed"
	ProcessingInProgress        ProcessingState = "in_progress"
	ProcessingStoppedResumable  ProcessingState = "stopped_resumable"
	ProcessingCompletedVerified ProcessingState = "completed_verified"
	ProcessingCompletedWithGaps ProcessingState = "completed_with_gaps"
	ProcessingFailed            ProcessingState = "failed"
	ProcessingMerged            ProcessingState = "merged"
)
