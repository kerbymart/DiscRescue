package recovery

import (
	"fmt"
	"sort"

	"discrescue/internal/mapfile"
)

const defaultAdaptiveThreshold uint32 = 32

type AdaptivePassMode string

const (
	AdaptivePassModeProbe AdaptivePassMode = "probe"
	AdaptivePassModeDone  AdaptivePassMode = "done"
)

type AdaptivePassDecisionKind string

const (
	AdaptivePassDecisionRead             AdaptivePassDecisionKind = "read"
	AdaptivePassDecisionWaitBackpressure AdaptivePassDecisionKind = "wait_backpressure"
	AdaptivePassDecisionDeferToScrape    AdaptivePassDecisionKind = "defer_to_scrape"
	AdaptivePassDecisionDone             AdaptivePassDecisionKind = "done"
)

type AdaptiveProbeDirection string

const (
	AdaptiveProbeLeftFirst  AdaptiveProbeDirection = "left_first"
	AdaptiveProbeRightFirst AdaptiveProbeDirection = "right_first"
)

type AdaptivePassDecision struct {
	Kind      AdaptivePassDecisionKind
	Request   Request
	Deferred  *mapfile.Extent
	Direction AdaptiveProbeDirection
}

type AdaptiveCandidate struct {
	Extent       mapfile.Extent
	SplitDepth   uint16
	HeadDistance uint64
}

type AdaptivePassState struct {
	Queue            []AdaptiveCandidate
	Threshold        uint32
	MaxSplitDepth    uint16
	HeadPosition     uint64
	Mode             AdaptivePassMode
	NextDirection    AdaptiveProbeDirection
	ActiveRequest    *Request
	ActiveDeferred   *mapfile.Extent
	ActiveDirection  AdaptiveProbeDirection
	ActiveCandidate  *AdaptiveCandidate
	DeferredToScrape []mapfile.Extent
}

type AdaptivePassOutcome struct {
	Request Request
	Success bool
}

func StartAdaptivePass(extents []mapfile.Extent, threshold uint32, maxSplitDepth uint16, headPosition uint64) (AdaptivePassState, error) {
	if err := mapfile.ValidateExtentSet(extents); err != nil {
		return AdaptivePassState{}, err
	}
	if threshold == 0 {
		return AdaptivePassState{}, fmt.Errorf("start adaptive pass: threshold must be greater than zero")
	}
	if maxSplitDepth == 0 {
		return AdaptivePassState{}, fmt.Errorf("start adaptive pass: max split depth must be greater than zero")
	}

	queue := make([]AdaptiveCandidate, 0, len(extents))
	for _, extent := range extents {
		if !schedulableAdaptiveState(extent.State) {
			continue
		}
		queue = append(queue, AdaptiveCandidate{
			Extent:       extent,
			SplitDepth:   0,
			HeadDistance: distanceToHead(extent, headPosition),
		})
	}
	sortAdaptiveQueue(queue)

	return AdaptivePassState{
		Queue:         queue,
		Threshold:     threshold,
		MaxSplitDepth: maxSplitDepth,
		HeadPosition:  headPosition,
		Mode:          AdaptivePassModeProbe,
		NextDirection: AdaptiveProbeLeftFirst,
	}, nil
}

func DispatchAdaptivePass(state AdaptivePassState, writerAvailable bool) (AdaptivePassState, AdaptivePassDecision, error) {
	if state.Threshold == 0 {
		return state, AdaptivePassDecision{}, fmt.Errorf("dispatch adaptive pass: threshold must be greater than zero")
	}
	if state.MaxSplitDepth == 0 {
		return state, AdaptivePassDecision{}, fmt.Errorf("dispatch adaptive pass: max split depth must be greater than zero")
	}
	if state.ActiveRequest != nil || state.ActiveDeferred != nil || state.ActiveCandidate != nil {
		return state, AdaptivePassDecision{}, fmt.Errorf("dispatch adaptive pass: previous action is still active")
	}
	if state.Mode == AdaptivePassModeDone {
		return state, AdaptivePassDecision{Kind: AdaptivePassDecisionDone}, nil
	}
	if !writerAvailable {
		return state, AdaptivePassDecision{Kind: AdaptivePassDecisionWaitBackpressure}, nil
	}
	if len(state.Queue) == 0 {
		next := state
		next.Mode = AdaptivePassModeDone
		return next, AdaptivePassDecision{Kind: AdaptivePassDecisionDone}, nil
	}
	refreshAdaptiveHeadDistances(state.Queue, state.HeadPosition)
	sortAdaptiveQueue(state.Queue)

	candidate := state.Queue[0]
	remaining := append([]AdaptiveCandidate(nil), state.Queue[1:]...)

	if candidate.Extent.Sectors < state.Threshold || candidate.SplitDepth >= state.MaxSplitDepth {
		next := state
		next.Queue = remaining
		deferred := candidate.Extent
		next.ActiveDeferred = &deferred
		next.ActiveCandidate = &candidate
		return next, AdaptivePassDecision{
			Kind:     AdaptivePassDecisionDeferToScrape,
			Deferred: &deferred,
		}, nil
	}

	request, direction, err := buildAdaptiveProbe(candidate, state.NextDirection)
	if err != nil {
		return state, AdaptivePassDecision{}, err
	}

	next := state
	next.Queue = remaining
	next.ActiveRequest = &request
	next.ActiveCandidate = &candidate
	next.ActiveDirection = direction
	return next, AdaptivePassDecision{
		Kind:      AdaptivePassDecisionRead,
		Request:   request,
		Direction: direction,
	}, nil
}

func ResolveAdaptivePass(state AdaptivePassState, outcome AdaptivePassOutcome) (AdaptivePassState, error) {
	if state.ActiveDeferred != nil {
		if state.ActiveCandidate == nil || *state.ActiveDeferred != state.ActiveCandidate.Extent {
			return state, fmt.Errorf("resolve adaptive pass: active deferred extent does not match active candidate")
		}
		next := state
		next.DeferredToScrape = append(next.DeferredToScrape, *state.ActiveDeferred)
		next.ActiveDeferred = nil
		next.ActiveCandidate = nil
		if len(next.Queue) == 0 {
			next.Mode = AdaptivePassModeDone
		}
		return next, nil
	}
	if state.ActiveRequest == nil || state.ActiveCandidate == nil {
		return state, fmt.Errorf("resolve adaptive pass: no active request")
	}
	if outcome.Request != *state.ActiveRequest {
		return state, fmt.Errorf("resolve adaptive pass: request %+v does not match active %+v", outcome.Request, *state.ActiveRequest)
	}

	next := state
	candidate := *state.ActiveCandidate
	next.ActiveRequest = nil
	next.ActiveCandidate = nil
	next.NextDirection = alternateAdaptiveDirection(state.ActiveDirection)
	next.HeadPosition = outcome.Request.StartLBA

	children, err := splitAdaptiveCandidate(candidate)
	if err != nil {
		return state, err
	}

	if outcome.Success {
		successful, remaining := orderAdaptiveChildren(children, state.ActiveDirection)
		successful.Extent.Attempts++
		if remaining != nil {
			remaining.Extent.Attempts++
			next.Queue = append(next.Queue, *remaining)
		}
		next.Queue = append(next.Queue, successful)
	} else {
		for _, child := range children {
			child.Extent.Attempts++
			next.Queue = append(next.Queue, child)
		}
	}

	sortAdaptiveQueue(next.Queue)
	if len(next.Queue) == 0 {
		next.Mode = AdaptivePassModeDone
	}
	return next, nil
}

func buildAdaptiveProbe(candidate AdaptiveCandidate, direction AdaptiveProbeDirection) (Request, AdaptiveProbeDirection, error) {
	children, err := splitAdaptiveCandidate(candidate)
	if err != nil {
		return Request{}, "", err
	}
	first, _ := orderAdaptiveChildren(children, direction)
	return AdaptivePass(first.Extent.StartLBA, first.Extent.Sectors), direction, nil
}

func splitAdaptiveCandidate(candidate AdaptiveCandidate) ([]AdaptiveCandidate, error) {
	midpoint := candidate.Extent.StartLBA + uint64(candidate.Extent.Sectors/2)
	left, right, err := mapfile.SplitExtent(candidate.Extent, midpoint)
	if err != nil {
		return nil, err
	}
	return []AdaptiveCandidate{
		{
			Extent:       left,
			SplitDepth:   candidate.SplitDepth + 1,
			HeadDistance: candidate.HeadDistance,
		},
		{
			Extent:       right,
			SplitDepth:   candidate.SplitDepth + 1,
			HeadDistance: candidate.HeadDistance,
		},
	}, nil
}

func orderAdaptiveChildren(children []AdaptiveCandidate, direction AdaptiveProbeDirection) (AdaptiveCandidate, *AdaptiveCandidate) {
	if len(children) != 2 {
		if len(children) == 1 {
			return children[0], nil
		}
		return AdaptiveCandidate{}, nil
	}
	if direction == AdaptiveProbeRightFirst {
		return children[1], &children[0]
	}
	return children[0], &children[1]
}

func sortAdaptiveQueue(queue []AdaptiveCandidate) {
	sort.Slice(queue, func(i, j int) bool {
		left := queue[i]
		right := queue[j]
		if left.Extent.Sectors != right.Extent.Sectors {
			return left.Extent.Sectors > right.Extent.Sectors
		}
		if left.Extent.Attempts != right.Extent.Attempts {
			return left.Extent.Attempts < right.Extent.Attempts
		}
		if left.HeadDistance != right.HeadDistance {
			return left.HeadDistance < right.HeadDistance
		}
		return left.Extent.StartLBA < right.Extent.StartLBA
	})
}

func refreshAdaptiveHeadDistances(queue []AdaptiveCandidate, headPosition uint64) {
	for i := range queue {
		queue[i].HeadDistance = distanceToHead(queue[i].Extent, headPosition)
	}
}

func distanceToHead(extent mapfile.Extent, head uint64) uint64 {
	if head < extent.StartLBA {
		return extent.StartLBA - head
	}
	if head >= extent.EndLBA() {
		return head - extent.EndLBA() + 1
	}
	return 0
}

func alternateAdaptiveDirection(direction AdaptiveProbeDirection) AdaptiveProbeDirection {
	if direction == AdaptiveProbeRightFirst {
		return AdaptiveProbeLeftFirst
	}
	return AdaptiveProbeRightFirst
}

func schedulableAdaptiveState(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateUnknown, mapfile.SectorStateSkipped, mapfile.SectorStateIOError, mapfile.SectorStateMissing, mapfile.SectorStateChecksumError:
		return true
	default:
		return false
	}
}
