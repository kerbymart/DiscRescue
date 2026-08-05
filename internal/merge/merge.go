package merge

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"

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

func BuildPlan(captures []Capture) (Plan, error) {
	if len(captures) == 0 {
		return Plan{}, fmt.Errorf("build merge plan: at least one capture is required")
	}

	baseline := captures[0].Identity
	if err := baseline.Validate(); err != nil {
		return Plan{}, fmt.Errorf("build merge plan baseline identity: %w", err)
	}

	boundaries := []uint64{0, baseline.SectorCount}
	for i, capture := range captures {
		if capture.CaptureID == "" {
			return Plan{}, fmt.Errorf("build merge plan capture[%d]: capture id is required", i)
		}
		if err := capture.Identity.Validate(); err != nil {
			return Plan{}, fmt.Errorf("build merge plan capture[%d] identity: %w", i, err)
		}
		if !reflect.DeepEqual(capture.Identity.MatchKey(), baseline.MatchKey()) {
			return Plan{}, fmt.Errorf("build merge plan capture[%d]: logical-content identity conflict blocks automatic merge", i)
		}
		if err := mapfile.ValidateExtentSet(capture.Extents); err != nil {
			return Plan{}, fmt.Errorf("build merge plan capture[%d] extents: %w", i, err)
		}
		for _, extent := range capture.Extents {
			if extent.EndLBA() > baseline.SectorCount {
				return Plan{}, fmt.Errorf(
					"build merge plan capture[%d]: extent [%d,%d) exceeds sector count %d",
					i,
					extent.StartLBA,
					extent.EndLBA(),
					baseline.SectorCount,
				)
			}
			boundaries = append(boundaries, extent.StartLBA, extent.EndLBA())
		}
	}

	sort.Slice(boundaries, func(i, j int) bool { return boundaries[i] < boundaries[j] })
	boundaries = uniqueBoundaries(boundaries)

	merged := make([]MergedExtent, 0, len(boundaries))
	for i := 0; i < len(boundaries)-1; i++ {
		start := boundaries[i]
		end := boundaries[i+1]
		if end <= start {
			continue
		}

		segment, err := resolveSegment(start, end, captures)
		if err != nil {
			return Plan{}, err
		}
		merged = appendMergedExtent(merged, segment)
	}

	return Plan{Extents: merged}, nil
}

type candidateExtent struct {
	CaptureID string
	Extent    mapfile.Extent
}

func resolveSegment(start, end uint64, captures []Capture) (MergedExtent, error) {
	candidates := make([]candidateExtent, 0, len(captures))
	for _, capture := range captures {
		if extent, _, ok := mapfile.LookupExtent(capture.Extents, start); ok && extent.EndLBA() >= end {
			candidates = append(candidates, candidateExtent{
				CaptureID: capture.CaptureID,
				Extent:    sliceExtent(extent, start, end),
			})
		}
	}

	hashes := uniqueHashes(candidates)
	if trusted, ok := filterTrustedVerified(candidates); ok {
		if !allSameHash(trusted) {
			return conflictExtent(start, end, hashes), nil
		}
		return mergedExtentFromCandidate(trusted[0], RuleTrustedChecksumMatch, mapfile.SectorStateVerified, mapfile.ConfidenceTrustedChecksum, hashes), nil
	}

	if reconstructed, ok := filterReconstructed(candidates); ok {
		if !allSameHash(reconstructed) {
			return conflictExtent(start, end, hashes), nil
		}
		return mergedExtentFromCandidate(reconstructed[0], RuleReconstructedTrustedChecksum, mapfile.SectorStateReconstructed, mapfile.ConfidenceReconstructedChecksum, hashes), nil
	}

	if verified, ok := filterVerified(candidates); ok {
		if len(verified) >= 2 && allSameHash(verified) {
			return mergedExtentFromCandidate(verified[0], RuleIdenticalVerifiedCandidates, mapfile.SectorStateVerified, mapfile.ConfidenceRepeatedIndependentCapture, hashes), nil
		}
		if len(verified) == 1 {
			return mergedExtentFromCandidate(verified[0], RuleSingleVerifiedCandidate, verified[0].Extent.State, verified[0].Extent.Confidence, hashes), nil
		}
		return conflictExtent(start, end, hashes), nil
	}

	if unverified, ok := filterUnverified(candidates); ok {
		if len(unverified) >= 2 && allSameHash(unverified) && distinctCaptures(unverified) >= 2 {
			return mergedExtentFromCandidate(unverified[0], RuleIdenticalUnverifiedCandidates, mapfile.SectorStateVerified, mapfile.ConfidenceRepeatedIndependentCapture, hashes), nil
		}
		if len(unverified) == 1 {
			return mergedExtentFromCandidate(unverified[0], RuleSingleUnverifiedCandidate, mapfile.SectorStateReadUnverified, mapfile.ConfidenceSingleRead, hashes), nil
		}
		return conflictExtent(start, end, hashes), nil
	}

	if len(hashes) > 0 {
		return conflictExtent(start, end, hashes), nil
	}

	return MergedExtent{
		Extent: mapfile.Extent{
			StartLBA:   start,
			Sectors:    uint32(end - start),
			State:      mapfile.SectorStateMissing,
			Confidence: mapfile.ConfidenceNone,
		},
		SelectionRule: RuleMissing,
		Provenance:    nil,
	}, nil
}

func sliceExtent(extent mapfile.Extent, start, end uint64) mapfile.Extent {
	next := extent
	next.StartLBA = start
	next.Sectors = uint32(end - start)
	return next
}

func filterTrustedVerified(candidates []candidateExtent) ([]candidateExtent, bool) {
	var filtered []candidateExtent
	for _, candidate := range candidates {
		if candidate.Extent.State == mapfile.SectorStateVerified && candidate.Extent.Confidence == mapfile.ConfidenceTrustedChecksum {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, len(filtered) > 0
}

func filterReconstructed(candidates []candidateExtent) ([]candidateExtent, bool) {
	var filtered []candidateExtent
	for _, candidate := range candidates {
		if candidate.Extent.State == mapfile.SectorStateReconstructed && candidate.Extent.Confidence == mapfile.ConfidenceReconstructedChecksum {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, len(filtered) > 0
}

func filterVerified(candidates []candidateExtent) ([]candidateExtent, bool) {
	var filtered []candidateExtent
	for _, candidate := range candidates {
		if candidate.Extent.State == mapfile.SectorStateVerified {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, len(filtered) > 0
}

func filterUnverified(candidates []candidateExtent) ([]candidateExtent, bool) {
	var filtered []candidateExtent
	for _, candidate := range candidates {
		if candidate.Extent.State == mapfile.SectorStateReadUnverified && candidate.Extent.Confidence == mapfile.ConfidenceSingleRead {
			filtered = append(filtered, candidate)
		}
	}
	return filtered, len(filtered) > 0
}

func allSameHash(candidates []candidateExtent) bool {
	if len(candidates) == 0 {
		return false
	}
	first := candidates[0].Extent.DataHash
	for _, candidate := range candidates[1:] {
		if candidate.Extent.DataHash != first {
			return false
		}
	}
	return true
}

func distinctCaptures(candidates []candidateExtent) int {
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		seen[candidate.CaptureID] = struct{}{}
	}
	return len(seen)
}

func uniqueHashes(candidates []candidateExtent) [][16]byte {
	seen := make(map[[16]byte]struct{}, len(candidates))
	hashes := make([][16]byte, 0, len(candidates))
	for _, candidate := range candidates {
		if !claimsData(candidate.Extent.State) {
			continue
		}
		if _, ok := seen[candidate.Extent.DataHash]; ok {
			continue
		}
		seen[candidate.Extent.DataHash] = struct{}{}
		hashes = append(hashes, candidate.Extent.DataHash)
	}
	sort.Slice(hashes, func(i, j int) bool {
		return bytes.Compare(hashes[i][:], hashes[j][:]) < 0
	})
	return hashes
}

func claimsData(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateReadUnverified, mapfile.SectorStateVerified, mapfile.SectorStateReconstructed:
		return true
	default:
		return false
	}
}

func mergedExtentFromCandidate(candidate candidateExtent, rule SelectionRule, state mapfile.SectorState, confidence mapfile.Confidence, hashes [][16]byte) MergedExtent {
	extent := candidate.Extent
	extent.State = state
	extent.Confidence = confidence
	return MergedExtent{
		Extent:            extent,
		SelectedCaptureID: candidate.CaptureID,
		SelectionRule:     rule,
		CandidateHashes:   append([][16]byte(nil), hashes...),
		Provenance:        provenanceForRule(rule, extent),
	}
}

func conflictExtent(start, end uint64, hashes [][16]byte) MergedExtent {
	return MergedExtent{
		Extent: mapfile.Extent{
			StartLBA:   start,
			Sectors:    uint32(end - start),
			State:      mapfile.SectorStateConflicting,
			Confidence: mapfile.ConfidenceNone,
		},
		SelectionRule:      RuleConflict,
		CandidateHashes:    append([][16]byte(nil), hashes...),
		Provenance:         integrity.NormalizeEvidence(integrity.EvidenceConflict),
		UnresolvedConflict: true,
	}
}

func appendMergedExtent(result []MergedExtent, next MergedExtent) []MergedExtent {
	if len(result) == 0 {
		return append(result, next)
	}
	last := result[len(result)-1]
	if canMergeMergedExtent(last, next) {
		last.Extent.Sectors += next.Extent.Sectors
		result[len(result)-1] = last
		return result
	}
	return append(result, next)
}

func canMergeMergedExtent(left, right MergedExtent) bool {
	return left.Extent.EndLBA() == right.Extent.StartLBA &&
		left.Extent.State == right.Extent.State &&
		left.Extent.Confidence == right.Extent.Confidence &&
		left.SelectedCaptureID == right.SelectedCaptureID &&
		left.SelectionRule == right.SelectionRule &&
		reflect.DeepEqual(left.Provenance, right.Provenance) &&
		left.UnresolvedConflict == right.UnresolvedConflict &&
		reflect.DeepEqual(left.CandidateHashes, right.CandidateHashes)
}

func uniqueBoundaries(boundaries []uint64) []uint64 {
	if len(boundaries) == 0 {
		return nil
	}
	out := []uint64{boundaries[0]}
	for _, value := range boundaries[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

func provenanceForRule(rule SelectionRule, extent mapfile.Extent) []integrity.Evidence {
	switch rule {
	case RuleTrustedChecksumMatch:
		return integrity.NormalizeEvidence(integrity.EvidenceTrustedChecksum)
	case RuleReconstructedTrustedChecksum:
		return integrity.NormalizeEvidence(integrity.EvidenceReconstruction, integrity.EvidenceTrustedChecksum)
	case RuleIdenticalVerifiedCandidates:
		return integrity.NormalizeEvidence(integrity.EvidenceRepeatedAgreement, integrity.EvidenceCrossCaptureAgree)
	case RuleSingleVerifiedCandidate:
		return integrity.EvidenceFromStateConfidence(extent)
	case RuleIdenticalUnverifiedCandidates:
		return integrity.NormalizeEvidence(integrity.EvidenceCrossCaptureAgree, integrity.EvidenceTentativeData)
	case RuleSingleUnverifiedCandidate:
		return integrity.NormalizeEvidence(integrity.EvidenceSuccessfulRead, integrity.EvidenceTentativeData)
	case RuleConflict:
		return integrity.NormalizeEvidence(integrity.EvidenceConflict)
	default:
		return nil
	}
}
