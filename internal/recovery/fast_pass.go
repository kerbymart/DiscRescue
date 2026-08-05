package recovery

import (
	"fmt"

	"discrescue/internal/mapfile"
)

const defaultFastSkipAfterFailure uint32 = 256

type FastPassMode string

const (
	FastPassModeCluster    FastPassMode = "cluster"
	FastPassModeSingle     FastPassMode = "single"
	FastPassModeCheckpoint FastPassMode = "checkpoint"
	FastPassModeDone       FastPassMode = "done"
)

type FastPassDecisionKind string

const (
	FastPassDecisionRead             FastPassDecisionKind = "read"
	FastPassDecisionWaitBackpressure FastPassDecisionKind = "wait_backpressure"
	FastPassDecisionCheckpoint       FastPassDecisionKind = "checkpoint"
	FastPassDecisionDone             FastPassDecisionKind = "done"
)

type FastPassDecision struct {
	Kind    FastPassDecisionKind
	Request Request
}

type FastPassState struct {
	Extents          []mapfile.Extent
	CursorLBA        uint64
	ClusterSize      uint32
	SkipAfterFailure uint32
	Mode             FastPassMode
	ClusterBoundary  uint64
	ActiveRequest    *Request
	CheckpointActive bool
}

type FastPassOutcome struct {
	Request             Request
	Success             bool
	CheckpointCompleted bool
}

func StartFastPass(extents []mapfile.Extent, clusterSize uint32) (FastPassState, error) {
	if err := mapfile.ValidateExtentSet(extents); err != nil {
		return FastPassState{}, err
	}
	if clusterSize == 0 {
		return FastPassState{}, fmt.Errorf("start fast pass: cluster size must be greater than zero")
	}
	return FastPassState{
		Extents:          append([]mapfile.Extent(nil), extents...),
		ClusterSize:      clusterSize,
		SkipAfterFailure: defaultFastSkipAfterFailure,
		Mode:             FastPassModeCluster,
	}, nil
}

func DispatchFastPass(state FastPassState, writerAvailable bool) (FastPassState, FastPassDecision, error) {
	if err := mapfile.ValidateExtentSet(state.Extents); err != nil {
		return state, FastPassDecision{}, err
	}
	if state.ClusterSize == 0 {
		return state, FastPassDecision{}, fmt.Errorf("dispatch fast pass: cluster size must be greater than zero")
	}
	if state.SkipAfterFailure == 0 {
		return state, FastPassDecision{}, fmt.Errorf("dispatch fast pass: skip-after-failure must be greater than zero")
	}
	if state.ActiveRequest != nil || state.CheckpointActive {
		return state, FastPassDecision{}, fmt.Errorf("dispatch fast pass: previous action is still active")
	}
	if state.Mode == FastPassModeDone {
		return state, FastPassDecision{Kind: FastPassDecisionDone}, nil
	}
	if state.Mode == FastPassModeCheckpoint {
		next := state
		next.CheckpointActive = true
		return next, FastPassDecision{Kind: FastPassDecisionCheckpoint}, nil
	}
	if !writerAvailable {
		return state, FastPassDecision{Kind: FastPassDecisionWaitBackpressure}, nil
	}

	request, ok := nextFastPassRequest(state)
	if !ok {
		next := state
		next.Mode = FastPassModeCheckpoint
		next.CheckpointActive = true
		return next, FastPassDecision{Kind: FastPassDecisionCheckpoint}, nil
	}

	next := state
	next.ActiveRequest = &request
	return next, FastPassDecision{Kind: FastPassDecisionRead, Request: request}, nil
}

func ResolveFastPass(state FastPassState, outcome FastPassOutcome) (FastPassState, error) {
	if state.CheckpointActive {
		if !outcome.CheckpointCompleted {
			return state, fmt.Errorf("resolve fast pass: checkpoint completion is required")
		}
		next := state
		next.CheckpointActive = false
		next.Mode = FastPassModeDone
		return next, nil
	}
	if state.ActiveRequest == nil {
		return state, fmt.Errorf("resolve fast pass: no active request")
	}
	if outcome.Request != *state.ActiveRequest {
		return state, fmt.Errorf("resolve fast pass: request %+v does not match active %+v", outcome.Request, *state.ActiveRequest)
	}

	request := *state.ActiveRequest
	next := state
	next.ActiveRequest = nil

	switch {
	case outcome.Success && state.Mode == FastPassModeCluster:
		next.CursorLBA = request.StartLBA + uint64(request.Sectors)
		next.Mode = FastPassModeCluster
		return next, nil
	case outcome.Success && state.Mode == FastPassModeSingle:
		next.CursorLBA = request.StartLBA + 1
		if next.CursorLBA < state.ClusterBoundary && hasSchedulableAtOrAfter(next.Extents, next.CursorLBA, state.ClusterBoundary) {
			next.Mode = FastPassModeSingle
			return next, nil
		}
		next.Mode = FastPassModeCluster
		if next.CursorLBA < state.ClusterBoundary {
			next.CursorLBA = state.ClusterBoundary
		}
		return next, nil
	case !outcome.Success && state.Mode == FastPassModeCluster:
		next.Mode = FastPassModeSingle
		next.CursorLBA = request.StartLBA
		next.ClusterBoundary = request.StartLBA + uint64(request.Sectors)
		return next, nil
	case !outcome.Success && state.Mode == FastPassModeSingle:
		skipSectors := skipLength(request.StartLBA+1, state.SkipAfterFailure, next.Extents)
		failed, err := applyFastPassFailure(next.Extents, request.StartLBA, state.SkipAfterFailure)
		if err != nil {
			return state, err
		}
		next.Extents = failed
		next.Mode = FastPassModeCluster
		next.CursorLBA = request.StartLBA + 1 + uint64(skipSectors)
		return next, nil
	default:
		return state, fmt.Errorf("resolve fast pass: unsupported outcome for mode %s", state.Mode)
	}
}

func nextFastPassRequest(state FastPassState) (Request, bool) {
	switch state.Mode {
	case FastPassModeCluster:
		start, sectors, ok := nextClusterRange(state.Extents, state.CursorLBA, state.ClusterSize)
		if !ok {
			return Request{}, false
		}
		return FastPass(start, sectors), true
	case FastPassModeSingle:
		start, ok := nextSingleSector(state.Extents, state.CursorLBA, state.ClusterBoundary)
		if !ok {
			return Request{}, false
		}
		return FastPass(start, 1), true
	default:
		return Request{}, false
	}
}

func nextClusterRange(extents []mapfile.Extent, cursor uint64, clusterSize uint32) (uint64, uint32, bool) {
	for _, extent := range extents {
		if !schedulableFastPassState(extent.State) || extent.EndLBA() <= cursor {
			continue
		}
		start := extent.StartLBA
		if start < cursor {
			start = cursor
		}
		span := uint32(extent.EndLBA() - start)
		if span > clusterSize {
			span = clusterSize
		}
		return start, span, true
	}
	return 0, 0, false
}

func nextSingleSector(extents []mapfile.Extent, cursor uint64, boundary uint64) (uint64, bool) {
	for _, extent := range extents {
		if !schedulableFastPassState(extent.State) || extent.EndLBA() <= cursor {
			continue
		}
		start := extent.StartLBA
		if start < cursor {
			start = cursor
		}
		if start >= boundary {
			return 0, false
		}
		return start, true
	}
	return 0, false
}

func hasSchedulableAtOrAfter(extents []mapfile.Extent, cursor uint64, boundary uint64) bool {
	_, ok := nextSingleSector(extents, cursor, boundary)
	return ok
}

func schedulableFastPassState(state mapfile.SectorState) bool {
	return state == mapfile.SectorStateUnknown
}

func applyFastPassFailure(extents []mapfile.Extent, failedLBA uint64, skipAfterFailure uint32) ([]mapfile.Extent, error) {
	failedExtents, err := replaceRangeState(extents, failedLBA, 1, mapfile.SectorStateIOError, mapfile.ConfidenceNone)
	if err != nil {
		return nil, err
	}
	skipStart := failedLBA + 1
	skipSectors := skipLength(skipStart, skipAfterFailure, failedExtents)
	if skipSectors == 0 {
		return failedExtents, nil
	}
	return replaceRangeState(failedExtents, skipStart, skipSectors, mapfile.SectorStateSkipped, mapfile.ConfidenceNone)
}

func skipLength(start uint64, limit uint32, extents []mapfile.Extent) uint32 {
	if limit == 0 {
		return 0
	}
	var total uint32
	current := start
	for _, extent := range extents {
		if !schedulableFastPassState(extent.State) || extent.EndLBA() <= current {
			continue
		}
		rangeStart := extent.StartLBA
		if rangeStart < current {
			rangeStart = current
		}
		if rangeStart != current {
			break
		}
		available := uint32(extent.EndLBA() - rangeStart)
		remaining := limit - total
		if available >= remaining {
			total += remaining
			return total
		}
		total += available
		current = extent.EndLBA()
		if total == limit {
			return total
		}
	}
	return total
}

func replaceRangeState(extents []mapfile.Extent, start uint64, sectors uint32, nextState mapfile.SectorState, confidence mapfile.Confidence) ([]mapfile.Extent, error) {
	if sectors == 0 {
		return append([]mapfile.Extent(nil), extents...), nil
	}
	end := start + uint64(sectors)
	result := make([]mapfile.Extent, 0, len(extents)+2)

	for _, extent := range extents {
		if extent.EndLBA() <= start || extent.StartLBA >= end {
			result = append(result, extent)
			continue
		}
		if !schedulableFastPassState(extent.State) {
			return nil, fmt.Errorf("replace range state: extent [%d,%d) is not schedulable", extent.StartLBA, extent.EndLBA())
		}
		if extent.StartLBA < start {
			left := extent
			left.Sectors = uint32(start - extent.StartLBA)
			result = append(result, left)
		}

		middleStart := maxUint64(extent.StartLBA, start)
		middleEnd := minUint64(extent.EndLBA(), end)
		middle := extent
		middle.StartLBA = middleStart
		middle.Sectors = uint32(middleEnd - middleStart)
		middle.State = nextState
		middle.Confidence = confidence
		middle.Attempts++
		result = append(result, middle)

		if middleEnd < extent.EndLBA() {
			right := extent
			right.StartLBA = middleEnd
			right.Sectors = uint32(extent.EndLBA() - middleEnd)
			result = append(result, right)
		}
	}

	return mapfile.CoalesceExtents(result)
}

func minUint64(left, right uint64) uint64 {
	if left < right {
		return left
	}
	return right
}

func maxUint64(left, right uint64) uint64 {
	if left > right {
		return left
	}
	return right
}
