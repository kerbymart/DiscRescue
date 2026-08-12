package mapfile

import (
	"fmt"
	"sort"
)

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
