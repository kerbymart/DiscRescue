package integrity

import (
	"fmt"

	"discrescue/internal/mapfile"
)

func VerifyExternal(input ExternalVerificationInput) (ExternalVerificationResult, error) {
	if input.LogicalSectorSize == 0 {
		return ExternalVerificationResult{}, fmt.Errorf("verify external: logical sector size must be greater than zero")
	}

	result := ExternalVerificationResult{
		Extents:    make([]mapfile.Extent, 0, len(input.Extents)),
		Provenance: make([]ExtentEvidence, 0, len(input.Extents)),
	}

	for i, item := range input.Extents {
		if err := item.Extent.Validate(); err != nil {
			return ExternalVerificationResult{}, fmt.Errorf("verify external extent[%d]: %w", i, err)
		}
		if !claimsImageData(item.Extent.State) {
			result.Extents = append(result.Extents, item.Extent)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: item.Extent.StartLBA,
				Sectors:  item.Extent.Sectors,
				Items:    EvidenceFromStateConfidence(item.Extent),
			})
			continue
		}

		expectedLength := uint64(item.Extent.Sectors) * uint64(input.LogicalSectorSize)
		if uint64(len(item.Data)) != expectedLength {
			return ExternalVerificationResult{}, fmt.Errorf(
				"verify external extent[%d]: data length %d does not match expected %d",
				i,
				len(item.Data),
				expectedLength,
			)
		}

		verified := false
		for _, digest := range item.Digests {
			ok, err := verifyDigest(item.Data, digest)
			if err != nil {
				return ExternalVerificationResult{}, fmt.Errorf("verify external extent[%d]: %w", i, err)
			}
			if !ok {
				continue
			}

			next := item.Extent
			if item.Extent.State == mapfile.SectorStateReconstructed {
				next.Confidence = mapfile.ConfidenceReconstructedChecksum
			} else {
				next.State = mapfile.SectorStateVerified
				next.Confidence = mapfile.ConfidenceTrustedChecksum
			}
			result.Extents = append(result.Extents, next)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: next.StartLBA,
				Sectors:  next.Sectors,
				Items:    EvidenceFromStateConfidence(next),
			})
			verified = true
			break
		}

		if verified {
			continue
		}

		downgraded := item.Extent
		downgraded.State = mapfile.SectorStateChecksumError
		downgraded.Confidence = mapfile.ConfidenceNone
		result.Extents = append(result.Extents, downgraded)
		result.Provenance = append(result.Provenance, ExtentEvidence{
			StartLBA: downgraded.StartLBA,
			Sectors:  downgraded.Sectors,
			Items:    nil,
		})
	}

	return result, nil
}
