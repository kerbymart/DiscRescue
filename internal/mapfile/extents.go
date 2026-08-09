package mapfile

import (
	"fmt"
	"math"
	"sort"
)

type Extent struct {
	StartLBA     uint64
	Sectors      uint32
	State        SectorState
	Confidence   Confidence
	Attempts     uint16
	CaptureID    uint32
	LastSenseKey uint8
	LastASC      uint8
	LastASCQ     uint8
	DataHash     [16]byte
}

func (e Extent) EndLBA() uint64 {
	return e.StartLBA + uint64(e.Sectors)
}

// CheckedEndLBA returns the exclusive end of an extent without allowing
// uint64 arithmetic to wrap.
func (e Extent) CheckedEndLBA() (uint64, error) {
	sectors := uint64(e.Sectors)
	if sectors > math.MaxUint64-e.StartLBA {
		return 0, fmt.Errorf("extent lba range overflows")
	}
	return e.StartLBA + sectors, nil
}

func (e Extent) Validate() error {
	if e.Sectors == 0 {
		return fmt.Errorf("validate extent: sectors must be greater than zero")
	}
	if _, err := e.CheckedEndLBA(); err != nil {
		return fmt.Errorf("validate extent: %w", err)
	}
	return ValidateStateConfidence(e.State, e.Confidence)
}

func (e Extent) Transition(nextState SectorState, nextConfidence Confidence) (Extent, error) {
	if err := ValidateTransition(e.State, e.Confidence, nextState, nextConfidence); err != nil {
		return Extent{}, err
	}

	next := e
	next.State = nextState
	next.Confidence = nextConfidence
	return next, nil
}

func Overlaps(left, right Extent) bool {
	return left.StartLBA < right.EndLBA() && right.StartLBA < left.EndLBA()
}

func CanMergeAdjacent(left, right Extent) bool {
	return left.EndLBA() == right.StartLBA &&
		left.State == right.State &&
		left.Confidence == right.Confidence &&
		left.Attempts == right.Attempts &&
		left.CaptureID == right.CaptureID &&
		left.LastSenseKey == right.LastSenseKey &&
		left.LastASC == right.LastASC &&
		left.LastASCQ == right.LastASCQ &&
		left.DataHash == right.DataHash
}

func MergeAdjacent(left, right Extent) (Extent, error) {
	if err := left.Validate(); err != nil {
		return Extent{}, fmt.Errorf("merge adjacent left: %w", err)
	}
	if err := right.Validate(); err != nil {
		return Extent{}, fmt.Errorf("merge adjacent right: %w", err)
	}
	if !CanMergeAdjacent(left, right) {
		return Extent{}, fmt.Errorf("merge adjacent: extents are not compatible neighbors")
	}

	merged := left
	merged.Sectors += right.Sectors
	return merged, nil
}

func SplitExtent(extent Extent, splitLBA uint64) (Extent, Extent, error) {
	if err := extent.Validate(); err != nil {
		return Extent{}, Extent{}, fmt.Errorf("split extent: %w", err)
	}
	if splitLBA <= extent.StartLBA || splitLBA >= extent.EndLBA() {
		return Extent{}, Extent{}, fmt.Errorf("split extent: split lba %d must be inside extent [%d,%d)", splitLBA, extent.StartLBA, extent.EndLBA())
	}

	left := extent
	left.Sectors = uint32(splitLBA - extent.StartLBA)

	right := extent
	right.StartLBA = splitLBA
	right.Sectors = uint32(extent.EndLBA() - splitLBA)

	return left, right, nil
}

func LookupExtent(extents []Extent, lba uint64) (Extent, int, bool) {
	for index, extent := range extents {
		if lba >= extent.StartLBA && lba < extent.EndLBA() {
			return extent, index, true
		}
	}
	return Extent{}, -1, false
}

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

func InsertExtent(extents []Extent, candidate Extent) ([]Extent, error) {
	if err := candidate.Validate(); err != nil {
		return nil, fmt.Errorf("insert extent candidate: %w", err)
	}
	if err := ValidateExtentSet(extents); err != nil {
		return nil, fmt.Errorf("insert extent set: %w", err)
	}

	inserted := false
	result := make([]Extent, 0, len(extents)+1)

	for _, extent := range extents {
		if Overlaps(extent, candidate) {
			return nil, fmt.Errorf("insert extent: candidate [%d,%d) overlaps [%d,%d)", candidate.StartLBA, candidate.EndLBA(), extent.StartLBA, extent.EndLBA())
		}
		if !inserted && candidate.EndLBA() <= extent.StartLBA {
			result = append(result, candidate)
			inserted = true
		}
		result = append(result, extent)
	}

	if !inserted {
		result = append(result, candidate)
	}

	return CoalesceExtents(result)
}

// ApplyExtent inserts a new extent or replaces the overlapping part of existing
// extents after validating every state transition. Existing non-overlapping
// tails are preserved, so journal records can safely refine deferred ranges
// without allowing stale work to overwrite recovered data.
func ApplyExtent(extents []Extent, candidate Extent) ([]Extent, error) {
	if err := candidate.Validate(); err != nil {
		return nil, fmt.Errorf("apply extent candidate: %w", err)
	}
	if err := ValidateExtentSet(extents); err != nil {
		return nil, fmt.Errorf("apply extent set: %w", err)
	}

	result := make([]Extent, 0, len(extents)+2)
	for _, current := range extents {
		if !Overlaps(current, candidate) {
			result = append(result, current)
			continue
		}

		if candidate.Attempts < current.Attempts {
			return nil, fmt.Errorf("apply extent: candidate attempts %d cannot replace newer extent attempts %d", candidate.Attempts, current.Attempts)
		}
		if err := ValidateTransition(current.State, current.Confidence, candidate.State, candidate.Confidence); err != nil {
			return nil, fmt.Errorf("apply extent [%d,%d) over [%d,%d): %w", candidate.StartLBA, candidate.EndLBA(), current.StartLBA, current.EndLBA(), err)
		}

		if current.StartLBA < candidate.StartLBA {
			left := current
			left.Sectors = uint32(candidate.StartLBA - current.StartLBA)
			result = append(result, left)
		}
		if current.EndLBA() > candidate.EndLBA() {
			right := current
			right.StartLBA = candidate.EndLBA()
			right.Sectors = uint32(current.EndLBA() - candidate.EndLBA())
			result = append(result, right)
		}
	}

	result = append(result, candidate)
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartLBA < result[j].StartLBA
	})
	return CoalesceExtents(result)
}

func CoalesceExtents(extents []Extent) ([]Extent, error) {
	if err := ValidateExtentSet(extents); err != nil {
		return nil, err
	}
	if len(extents) == 0 {
		return nil, nil
	}

	result := make([]Extent, 0, len(extents))
	current := extents[0]

	for _, next := range extents[1:] {
		if CanMergeAdjacent(current, next) {
			merged, err := MergeAdjacent(current, next)
			if err != nil {
				return nil, err
			}
			current = merged
			continue
		}

		result = append(result, current)
		current = next
	}

	result = append(result, current)
	return result, nil
}
