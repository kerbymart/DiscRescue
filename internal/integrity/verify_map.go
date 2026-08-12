package integrity

import (
	"fmt"

	"discrescue/internal/mapfile"
)

func VerifyMap(input MapVerificationInput) (MapVerificationResult, error) {
	header, err := mapfile.UnmarshalHeader(input.HeaderBytes)
	if err != nil {
		return MapVerificationResult{}, fmt.Errorf("verify map header: %w", err)
	}

	checkpoint, err := mapfile.UnmarshalCheckpoint(input.CheckpointBytes)
	if err != nil {
		return MapVerificationResult{}, fmt.Errorf("verify map checkpoint: %w", err)
	}

	replayed, err := mapfile.ReplayJournalWithinCapacity(checkpoint, input.JournalBytes, header.ExpectedSectorCount)
	if err != nil {
		return MapVerificationResult{}, fmt.Errorf("verify map journal: %w", err)
	}

	requiredImageBytes, err := requiredImageBytes(replayed.Extents, header.LogicalSectorSize)
	if err != nil {
		return MapVerificationResult{}, fmt.Errorf("verify map image offset: %w", err)
	}
	if input.ImageLength < requiredImageBytes {
		return MapVerificationResult{}, fmt.Errorf(
			"verify map image length: image length %d is smaller than required %d",
			input.ImageLength,
			requiredImageBytes,
		)
	}

	return MapVerificationResult{
		Header:             header,
		ReplayedCheckpoint: replayed,
		RequiredImageBytes: requiredImageBytes,
	}, nil
}
func requiredImageBytes(extents []mapfile.Extent, logicalSectorSize uint32) (uint64, error) {
	var maxEnd uint64
	for _, extent := range extents {
		if !claimsImageData(extent.State) {
			continue
		}
		end, err := extent.CheckedEndLBA()
		if err != nil {
			return 0, err
		}
		if end > maxEnd {
			maxEnd = end
		}
	}
	return mapfile.CheckedSectorByteOffset(maxEnd, logicalSectorSize)
}
func claimsImageData(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateReadUnverified,
		mapfile.SectorStateVerified,
		mapfile.SectorStateChecksumError,
		mapfile.SectorStateConflicting,
		mapfile.SectorStateReconstructed:
		return true
	default:
		return false
	}
}
