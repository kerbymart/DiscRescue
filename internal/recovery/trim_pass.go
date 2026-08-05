package recovery

import (
	"fmt"

	"discrescue/internal/mapfile"
)

const defaultTrimSmallCluster uint32 = 4

type TrimPassMode string

const (
	TrimPassModeLeftCluster  TrimPassMode = "left_cluster"
	TrimPassModeLeftSingle   TrimPassMode = "left_single"
	TrimPassModeRightCluster TrimPassMode = "right_cluster"
	TrimPassModeRightSingle  TrimPassMode = "right_single"
	TrimPassModeDone         TrimPassMode = "done"
)

type TrimPassDecisionKind string

const (
	TrimPassDecisionRead             TrimPassDecisionKind = "read"
	TrimPassDecisionWaitBackpressure TrimPassDecisionKind = "wait_backpressure"
	TrimPassDecisionDone             TrimPassDecisionKind = "done"
)

type TrimPassDecision struct {
	Kind    TrimPassDecisionKind
	Request Request
}

type TrimPassState struct {
	Target             mapfile.Extent
	SmallCluster       uint32
	FailureBudget      uint16
	LeftCursor         uint64
	RightCursor        uint64
	Mode               TrimPassMode
	ActiveRequest      *Request
	LeftFailureStreak  uint16
	RightFailureStreak uint16
}

type TrimPassOutcome struct {
	Request Request
	Success bool
}

func StartTrimPass(target mapfile.Extent, smallCluster uint32, failureBudget uint16) (TrimPassState, error) {
	if err := target.Validate(); err != nil {
		return TrimPassState{}, err
	}
	if !schedulableTrimState(target.State) {
		return TrimPassState{}, fmt.Errorf("start trim pass: extent state %s is not unresolved", target.State)
	}
	if smallCluster == 0 {
		return TrimPassState{}, fmt.Errorf("start trim pass: small cluster must be greater than zero")
	}
	if failureBudget == 0 {
		return TrimPassState{}, fmt.Errorf("start trim pass: failure budget must be greater than zero")
	}

	return TrimPassState{
		Target:        target,
		SmallCluster:  smallCluster,
		FailureBudget: failureBudget,
		LeftCursor:    target.StartLBA,
		RightCursor:   target.EndLBA(),
		Mode:          TrimPassModeLeftCluster,
	}, nil
}

func DispatchTrimPass(state TrimPassState, writerAvailable bool) (TrimPassState, TrimPassDecision, error) {
	if err := state.Target.Validate(); err != nil {
		return state, TrimPassDecision{}, err
	}
	if state.SmallCluster == 0 {
		return state, TrimPassDecision{}, fmt.Errorf("dispatch trim pass: small cluster must be greater than zero")
	}
	if state.FailureBudget == 0 {
		return state, TrimPassDecision{}, fmt.Errorf("dispatch trim pass: failure budget must be greater than zero")
	}
	if state.ActiveRequest != nil {
		return state, TrimPassDecision{}, fmt.Errorf("dispatch trim pass: previous request is still active")
	}
	if state.Mode == TrimPassModeDone {
		return state, TrimPassDecision{Kind: TrimPassDecisionDone}, nil
	}
	if !writerAvailable {
		return state, TrimPassDecision{Kind: TrimPassDecisionWaitBackpressure}, nil
	}

	request, ok := nextTrimPassRequest(state)
	if !ok {
		next := state
		next.Mode = TrimPassModeDone
		return next, TrimPassDecision{Kind: TrimPassDecisionDone}, nil
	}

	next := state
	next.ActiveRequest = &request
	return next, TrimPassDecision{Kind: TrimPassDecisionRead, Request: request}, nil
}

func ResolveTrimPass(state TrimPassState, outcome TrimPassOutcome) (TrimPassState, error) {
	if state.ActiveRequest == nil {
		return state, fmt.Errorf("resolve trim pass: no active request")
	}
	if outcome.Request != *state.ActiveRequest {
		return state, fmt.Errorf("resolve trim pass: request %+v does not match active %+v", outcome.Request, *state.ActiveRequest)
	}

	request := *state.ActiveRequest
	next := state
	next.ActiveRequest = nil

	switch state.Mode {
	case TrimPassModeLeftCluster:
		if outcome.Success {
			next.LeftCursor += uint64(request.Sectors)
			return finishOrContinueTrim(next, TrimPassModeLeftCluster), nil
		}
		next.Mode = TrimPassModeLeftSingle
		next.LeftFailureStreak = 0
		return next, nil

	case TrimPassModeLeftSingle:
		if outcome.Success {
			next.LeftCursor++
			next.LeftFailureStreak = 0
			return finishOrContinueTrim(next, TrimPassModeLeftSingle), nil
		}
		next.LeftFailureStreak++
		if next.LeftFailureStreak >= next.FailureBudget {
			next.Mode = nextRightMode(next)
			next.LeftFailureStreak = 0
		}
		return finishOrContinueTrim(next, next.Mode), nil

	case TrimPassModeRightCluster:
		if outcome.Success {
			next.RightCursor -= uint64(request.Sectors)
			return finishOrContinueTrim(next, TrimPassModeRightCluster), nil
		}
		next.Mode = TrimPassModeRightSingle
		next.RightFailureStreak = 0
		return next, nil

	case TrimPassModeRightSingle:
		if outcome.Success {
			next.RightCursor--
			next.RightFailureStreak = 0
			return finishOrContinueTrim(next, TrimPassModeRightSingle), nil
		}
		next.RightFailureStreak++
		if next.RightFailureStreak >= next.FailureBudget {
			next.Mode = TrimPassModeDone
			next.RightFailureStreak = 0
		}
		return finishOrContinueTrim(next, next.Mode), nil

	default:
		return state, fmt.Errorf("resolve trim pass: unsupported mode %s", state.Mode)
	}
}

func RemainingTrimInterior(state TrimPassState) *mapfile.Extent {
	if state.LeftCursor >= state.RightCursor {
		return nil
	}
	remaining := state.Target
	remaining.StartLBA = state.LeftCursor
	remaining.Sectors = uint32(state.RightCursor - state.LeftCursor)
	return &remaining
}

func nextTrimPassRequest(state TrimPassState) (Request, bool) {
	if state.LeftCursor >= state.RightCursor {
		return Request{}, false
	}

	switch state.Mode {
	case TrimPassModeLeftCluster:
		sectors := uint32(state.RightCursor - state.LeftCursor)
		if sectors > state.SmallCluster {
			sectors = state.SmallCluster
		}
		return TrimPass(state.LeftCursor, sectors), true
	case TrimPassModeLeftSingle:
		return TrimPass(state.LeftCursor, 1), true
	case TrimPassModeRightCluster:
		sectors := uint32(state.RightCursor - state.LeftCursor)
		if sectors > state.SmallCluster {
			sectors = state.SmallCluster
		}
		return TrimPass(state.RightCursor-uint64(sectors), sectors), true
	case TrimPassModeRightSingle:
		return TrimPass(state.RightCursor-1, 1), true
	default:
		return Request{}, false
	}
}

func finishOrContinueTrim(state TrimPassState, preferred TrimPassMode) TrimPassState {
	if state.LeftCursor >= state.RightCursor {
		state.Mode = TrimPassModeDone
		return state
	}
	state.Mode = preferred
	return state
}

func nextRightMode(state TrimPassState) TrimPassMode {
	if state.LeftCursor >= state.RightCursor {
		return TrimPassModeDone
	}
	return TrimPassModeRightCluster
}

func schedulableTrimState(state mapfile.SectorState) bool {
	switch state {
	case mapfile.SectorStateUnknown, mapfile.SectorStateSkipped, mapfile.SectorStateIOError, mapfile.SectorStateMissing, mapfile.SectorStateChecksumError:
		return true
	default:
		return false
	}
}
