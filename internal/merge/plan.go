package merge

import (
	"fmt"
	"reflect"
	"sort"

	"discrescue/internal/mapfile"
)

func BuildPlan(captures []Capture) (Plan, error) {
	if len(captures) == 0 {
		return Plan{}, fmt.Errorf("build merge plan: at least one capture is required")
	}

	baseline := captures[0].Identity
	if err := baseline.Validate(); err != nil {
		return Plan{}, fmt.Errorf("build merge plan baseline identity: %w", err)
	}

	boundaries := []uint64{0, baseline.SectorCount}
	for i, capture := range captures {
		if capture.CaptureID == "" {
			return Plan{}, fmt.Errorf("build merge plan capture[%d]: capture id is required", i)
		}
		if err := capture.Identity.Validate(); err != nil {
			return Plan{}, fmt.Errorf("build merge plan capture[%d] identity: %w", i, err)
		}
		if !reflect.DeepEqual(capture.Identity.MatchKey(), baseline.MatchKey()) {
			return Plan{}, fmt.Errorf("build merge plan capture[%d]: logical-content identity conflict blocks automatic merge", i)
		}
		if err := mapfile.ValidateExtentSet(capture.Extents); err != nil {
			return Plan{}, fmt.Errorf("build merge plan capture[%d] extents: %w", i, err)
		}
		for _, extent := range capture.Extents {
			if extent.EndLBA() > baseline.SectorCount {
				return Plan{}, fmt.Errorf(
					"build merge plan capture[%d]: extent [%d,%d) exceeds sector count %d",
					i,
					extent.StartLBA,
					extent.EndLBA(),
					baseline.SectorCount,
				)
			}
			boundaries = append(boundaries, extent.StartLBA, extent.EndLBA())
		}
	}

	sort.Slice(boundaries, func(i, j int) bool { return boundaries[i] < boundaries[j] })
	boundaries = uniqueBoundaries(boundaries)

	merged := make([]MergedExtent, 0, len(boundaries))
	for i := 0; i < len(boundaries)-1; i++ {
		start := boundaries[i]
		end := boundaries[i+1]
		if end <= start {
			continue
		}

		segment, err := resolveSegment(start, end, captures)
		if err != nil {
			return Plan{}, err
		}
		merged = appendMergedExtent(merged, segment)
	}

	return Plan{Extents: merged}, nil
}

func uniqueBoundaries(boundaries []uint64) []uint64 {
	if len(boundaries) == 0 {
		return nil
	}
	out := []uint64{boundaries[0]}
	for _, value := range boundaries[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
