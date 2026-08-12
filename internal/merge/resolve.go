package merge

import (
	"discrescue/internal/integrity"
	"discrescue/internal/mapfile"
)

type candidateExtent struct {
	CaptureID string
	Extent    mapfile.Extent
}

func resolveSegment(start, end uint64, captures []Capture) (MergedExtent, error) {
	candidates := make([]candidateExtent, 0, len(captures))
	for _, capture := range captures {
		if extent, _, ok := mapfile.LookupExtent(capture.Extents, start); ok && extent.EndLBA() >= end {
			candidates = append(candidates, candidateExtent{CaptureID: capture.CaptureID, Extent: sliceExtent(extent, start, end)})
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
		Extent: mapfile.Extent{StartLBA: start, Sectors: uint32(end - start), State: mapfile.SectorStateMissing, Confidence: mapfile.ConfidenceNone},
		SelectionRule: RuleMissing,
	}, nil
}

func sliceExtent(extent mapfile.Extent, start, end uint64) mapfile.Extent {
	next := extent
	next.StartLBA = start
	next.Sectors = uint32(end - start)
	return next
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
		Extent:             mapfile.Extent{StartLBA: start, Sectors: uint32(end - start), State: mapfile.SectorStateConflicting, Confidence: mapfile.ConfidenceNone},
		SelectionRule:      RuleConflict,
		CandidateHashes:    append([][16]byte(nil), hashes...),
		Provenance:         integrity.NormalizeEvidence(integrity.EvidenceConflict),
		UnresolvedConflict: true,
	}
}
