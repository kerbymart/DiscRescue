package mapfile

import (
	"fmt"
	"math"
)

func ValidateExtentSet(extents []Extent) error {
	for index, extent := range extents {
		if err := extent.Validate(); err != nil {
			return fmt.Errorf("validate extent set[%d]: %w", index, err)
		}
		if index == 0 {
			continue
		}
		previous := extents[index-1]
		if previous.StartLBA > extent.StartLBA {
			return fmt.Errorf("validate extent set[%d]: extents are not sorted", index)
		}
		if Overlaps(previous, extent) {
			return fmt.Errorf("validate extent set[%d]: extents overlap", index)
		}
	}
	return nil
}

// ValidateExtentSetWithinCapacity validates intrinsic extent invariants and
// requires every extent to be contained in the declared media geometry.
func ValidateExtentSetWithinCapacity(extents []Extent, expectedSectorCount uint64) error {
	if expectedSectorCount == 0 {
		return fmt.Errorf("validate extent capacity: expected sector count must be greater than zero")
	}
	if err := ValidateExtentSet(extents); err != nil {
		return err
	}
	for index, extent := range extents {
		end, err := extent.CheckedEndLBA()
		if err != nil {
			return fmt.Errorf("validate extent capacity[%d]: %w", index, err)
		}
		if extent.StartLBA >= expectedSectorCount || end > expectedSectorCount {
			return fmt.Errorf("validate extent capacity[%d]: range [%d,%d) exceeds media capacity %d", index, extent.StartLBA, end, expectedSectorCount)
		}
	}
	return nil
}

// CheckedSectorByteOffset converts a sector position to an image byte offset
// without allowing multiplication to wrap.
func CheckedSectorByteOffset(lba uint64, logicalSectorSize uint32) (uint64, error) {
	if logicalSectorSize == 0 {
		return 0, fmt.Errorf("sector byte offset: logical sector size must be greater than zero")
	}
	sectorSize := uint64(logicalSectorSize)
	if lba > math.MaxUint64/sectorSize {
		return 0, fmt.Errorf("sector byte offset: lba %d overflows sector size %d", lba, logicalSectorSize)
	}
	return lba * sectorSize, nil
}
