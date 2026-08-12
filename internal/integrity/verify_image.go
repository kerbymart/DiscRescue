package integrity

import (
	"fmt"

	"discrescue/internal/mapfile"
)

func VerifyImage(input ImageVerificationInput) (ImageVerificationResult, error) {
	if input.LogicalSectorSize == 0 {
		return ImageVerificationResult{}, fmt.Errorf("verify image: logical sector size must be greater than zero")
	}

	result := ImageVerificationResult{
		Extents:    make([]mapfile.Extent, 0, len(input.Extents)),
		Provenance: make([]ExtentEvidence, 0, len(input.Extents)),
	}

	for i, item := range input.Extents {
		if err := item.Extent.Validate(); err != nil {
			return ImageVerificationResult{}, fmt.Errorf("verify image extent[%d]: %w", i, err)
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
			return ImageVerificationResult{}, fmt.Errorf(
				"verify image extent[%d]: data length %d does not match expected %d",
				i,
				len(item.Data),
				expectedLength,
			)
		}

		if item.Extent.DataHash == ([16]byte{}) {
			result.Extents = append(result.Extents, item.Extent)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: item.Extent.StartLBA,
				Sectors:  item.Extent.Sectors,
				Items:    EvidenceFromStateConfidence(item.Extent),
			})
			continue
		}

		if hash16(item.Data) == item.Extent.DataHash {
			result.Extents = append(result.Extents, item.Extent)
			result.Provenance = append(result.Provenance, ExtentEvidence{
				StartLBA: item.Extent.StartLBA,
				Sectors:  item.Extent.Sectors,
				Items:    EvidenceFromStateConfidence(item.Extent),
			})
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
		offset, err := mapfile.CheckedSectorByteOffset(item.Extent.StartLBA, input.LogicalSectorSize)
		if err != nil {
			return ImageVerificationResult{}, fmt.Errorf("verify image extent[%d]: %w", i, err)
		}
		result.ChangedRanges = append(result.ChangedRanges, ByteRange{
			Offset: offset,
			Length: expectedLength,
		})
	}

	return result, nil
}
