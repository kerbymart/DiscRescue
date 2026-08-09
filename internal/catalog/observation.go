package catalog

import "time"

// IdentityStatus describes how much trustworthy evidence was collected from
// the currently inserted media.
type IdentityStatus string

const (
	IdentityStrongEvidence       IdentityStatus = "strong_evidence"
	IdentityPartialEvidence      IdentityStatus = "partial_evidence"
	IdentityInsufficientEvidence IdentityStatus = "insufficient_evidence"
	IdentityUnavailable          IdentityStatus = "unavailable"
)

// IdentityObservation combines identity evidence with bounded collection
// accounting. It is safe to display without implying that matching succeeded.
type IdentityObservation struct {
	Identity           ContentIdentity
	Status             IdentityStatus
	AttemptedSamples   uint16
	AvailableSamples   uint16
	UnavailableSamples uint16
	BytesRead          uint64
	CollectionDuration time.Duration
	Detail             string
}

// PriorProcessingKind is the application-neutral result of catalog lookup.
type PriorProcessingKind string

const (
	PriorNone          PriorProcessingKind = "none"
	PriorStrongResume  PriorProcessingKind = "strong_resume"
	PriorStrongDone    PriorProcessingKind = "strong_completed"
	PriorProbable      PriorProcessingKind = "probable"
	PriorIndeterminate PriorProcessingKind = "indeterminate"
	PriorConflict      PriorProcessingKind = "conflict"
	PriorUnavailable   PriorProcessingKind = "files_unavailable"
)

// PriorProcessingResult is the only result consumers should use to decide
// whether prior work may be presented or resumed.
type PriorProcessingResult struct {
	Kind              PriorProcessingKind
	Match             MatchStrength
	Candidates        []Entry
	AutoResumeAllowed bool
	Detail            string
}
