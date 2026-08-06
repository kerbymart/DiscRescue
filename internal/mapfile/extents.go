package mapfile

import "fmt"

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

func (e Extent) Validate() error {
	if e.Sectors == 0 {
		return fmt.Errorf("validate extent: sectors must be greater than zero")
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

func ApplyExtent(extents []Extent, candidate Extent) ([]Extent, error) {
	if err := candidate.Validate(); err != nil {
		return nil, fmt.Errorf("apply extent candidate: %w", err)
	}
	if err := ValidateExtentSet(extents); err != nil {
		return nil, fmt.Errorf("apply extent set: %w", err)
	}

	result := make([]Extent, 0, len(extents)+2)
	inserted := false

	for _, extent := range extents {
		if extent.EndLBA() <= candidate.StartLBA || extent.StartLBA >= candidate.EndLBA() {
			if !inserted && candidate.EndLBA() <= extent.StartLBA {
				result = append(result, candidate)
				inserted = true
			}
			result = append(result, extent)
			continue
		}

		if extent.StartLBA < candidate.StartLBA {
			left := extent
			left.Sectors = uint32(candidate.StartLBA - extent.StartLBA)
			result = append(result, left)
		}
		if !inserted {
			result = append(result, candidate)
			inserted = true
		}
		if candidate.EndLBA() < extent.EndLBA() {
			right := extent
			right.StartLBA = candidate.EndLBA()
			right.Sectors = uint32(extent.EndLBA() - candidate.EndLBA())
			result = append(result, right)
		}
	}

	if !inserted {
		result = append(result, candidate)
	}

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
