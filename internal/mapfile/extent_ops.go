package mapfile

import "fmt"

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
