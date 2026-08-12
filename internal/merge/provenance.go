package merge

import (
	"discrescue/internal/integrity"
	"discrescue/internal/mapfile"
)

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
