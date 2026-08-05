package integrity

import "discrescue/internal/mapfile"

type Evidence string

const (
	EvidenceSuccessfulRead    Evidence = "successful_read"
	EvidenceRepeatedAgreement Evidence = "repeated_agreement"
	EvidenceCrossCaptureAgree Evidence = "cross_capture_agreement"
	EvidenceTrustedChecksum   Evidence = "trusted_checksum"
	EvidenceReconstruction    Evidence = "reconstruction"
	EvidenceTentativeData     Evidence = "tentative_data"
	EvidenceConflict          Evidence = "conflict"
)

type ExtentEvidence struct {
	StartLBA uint64
	Sectors  uint32
	Items    []Evidence
}

func NormalizeEvidence(items ...Evidence) []Evidence {
	seen := map[Evidence]struct{}{}
	out := make([]Evidence, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func EvidenceFromStateConfidence(extent mapfile.Extent) []Evidence {
	switch extent.State {
	case mapfile.SectorStateReadUnverified:
		return NormalizeEvidence(EvidenceSuccessfulRead, EvidenceTentativeData)
	case mapfile.SectorStateVerified:
		switch extent.Confidence {
		case mapfile.ConfidenceRepeatedSingleCapture:
			return NormalizeEvidence(EvidenceRepeatedAgreement)
		case mapfile.ConfidenceRepeatedIndependentCapture:
			return NormalizeEvidence(EvidenceRepeatedAgreement, EvidenceCrossCaptureAgree)
		case mapfile.ConfidenceTrustedChecksum:
			return NormalizeEvidence(EvidenceTrustedChecksum)
		}
	case mapfile.SectorStateReconstructed:
		return NormalizeEvidence(EvidenceReconstruction, EvidenceTrustedChecksum)
	case mapfile.SectorStateConflicting:
		return NormalizeEvidence(EvidenceConflict)
	}
	return nil
}
