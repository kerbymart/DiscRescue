package merge

import (
	"discrescue/internal/catalog"
	"discrescue/internal/integrity"
	"discrescue/internal/mapfile"
)

type SelectionRule string

const (
	RuleTrustedChecksumMatch          SelectionRule = "trusted_checksum_match"
	RuleReconstructedTrustedChecksum  SelectionRule = "reconstructed_trusted_checksum"
	RuleIdenticalVerifiedCandidates   SelectionRule = "identical_verified_candidates"
	RuleSingleVerifiedCandidate       SelectionRule = "single_verified_candidate"
	RuleIdenticalUnverifiedCandidates SelectionRule = "identical_unverified_candidates"
	RuleSingleUnverifiedCandidate     SelectionRule = "single_unverified_candidate"
	RuleConflict                      SelectionRule = "conflict"
	RuleMissing                       SelectionRule = "missing"
)

type Capture struct {
	CaptureID string
	Identity  catalog.ContentIdentity
	Extents   []mapfile.Extent
}

type MergedExtent struct {
	Extent             mapfile.Extent
	SelectedCaptureID  string
	SelectionRule      SelectionRule
	CandidateHashes    [][16]byte
	Provenance         []integrity.Evidence
	UnresolvedConflict bool
}

type Plan struct {
	Extents []MergedExtent
}
