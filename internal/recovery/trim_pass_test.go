package recovery

import (
	"testing"

	"discrescue/internal/mapfile"
)

func TestDispatchTrimPassIssuesLeftEdgeClusterRead(t *testing.T) {
	state := mustStartTrimPass(t, mapfile.Extent{
		StartLBA:   100,
		Sectors:    20,
		State:      mapfile.SectorStateSkipped,
		Confidence: mapfile.ConfidenceNone,
	}, 4, 2)

	next, decision, err := DispatchTrimPass(state, true)
	if err != nil {
		t.Fatalf("dispatch trim pass: %v", err)
	}
	if decision.Kind != TrimPassDecisionRead {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if decision.Request.StartLBA != 100 || decision.Request.Sectors != 4 || decision.Request.Strategy != StrategyTrim {
		t.Fatalf("unexpected request: %+v", decision.Request)
	}
	if next.ActiveRequest == nil {
		t.Fatal("expected active request to be tracked")
	}
}

func TestDispatchTrimPassWaitsForWriterBackpressure(t *testing.T) {
	state := mustStartTrimPass(t, mapfile.Extent{
		StartLBA:   100,
		Sectors:    20,
		State:      mapfile.SectorStateSkipped,
		Confidence: mapfile.ConfidenceNone,
	}, 4, 2)

	next, decision, err := DispatchTrimPass(state, false)
	if err != nil {
		t.Fatalf("dispatch trim pass: %v", err)
	}
	if decision.Kind != TrimPassDecisionWaitBackpressure {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if next.ActiveRequest != nil {
		t.Fatal("did not expect an active request while backpressured")
	}
}

func TestResolveTrimPassClusterFailureFallsBackToSingleSector(t *testing.T) {
	state := mustStartTrimPass(t, mapfile.Extent{
		StartLBA:   100,
		Sectors:    20,
		State:      mapfile.SectorStateSkipped,
		Confidence: mapfile.ConfidenceNone,
	}, 4, 2)

	active, decision, err := DispatchTrimPass(state, true)
	if err != nil {
		t.Fatalf("dispatch trim pass: %v", err)
	}
	failed, err := ResolveTrimPass(active, TrimPassOutcome{Request: decision.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve trim pass: %v", err)
	}
	if failed.Mode != TrimPassModeLeftSingle {
		t.Fatalf("unexpected mode after cluster failure: %s", failed.Mode)
	}

	_, fallback, err := DispatchTrimPass(failed, true)
	if err != nil {
		t.Fatalf("dispatch single fallback: %v", err)
	}
	if fallback.Request.StartLBA != 100 || fallback.Request.Sectors != 1 {
		t.Fatalf("unexpected fallback request: %+v", fallback.Request)
	}
}

func TestResolveTrimPassSingleFailureRetriesUntilBudgetThenSwitchesRight(t *testing.T) {
	state := mustStartTrimPass(t, mapfile.Extent{
		StartLBA:   100,
		Sectors:    20,
		State:      mapfile.SectorStateSkipped,
		Confidence: mapfile.ConfidenceNone,
	}, 4, 2)

	active, cluster, err := DispatchTrimPass(state, true)
	if err != nil {
		t.Fatalf("dispatch cluster: %v", err)
	}
	singleState, err := ResolveTrimPass(active, TrimPassOutcome{Request: cluster.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve cluster failure: %v", err)
	}

	active, single, err := DispatchTrimPass(singleState, true)
	if err != nil {
		t.Fatalf("dispatch single: %v", err)
	}
	retryState, err := ResolveTrimPass(active, TrimPassOutcome{Request: single.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve first single failure: %v", err)
	}
	if retryState.Mode != TrimPassModeLeftSingle || retryState.LeftFailureStreak != 1 {
		t.Fatalf("unexpected retry state: %+v", retryState)
	}

	active, retry, err := DispatchTrimPass(retryState, true)
	if err != nil {
		t.Fatalf("dispatch retry: %v", err)
	}
	if retry.Request.StartLBA != 100 || retry.Request.Sectors != 1 {
		t.Fatalf("unexpected retry request: %+v", retry.Request)
	}
	rightState, err := ResolveTrimPass(active, TrimPassOutcome{Request: retry.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve second single failure: %v", err)
	}
	if rightState.Mode != TrimPassModeRightCluster {
		t.Fatalf("expected switch to right cluster after budget exhaustion, got %s", rightState.Mode)
	}

	_, rightDecision, err := DispatchTrimPass(rightState, true)
	if err != nil {
		t.Fatalf("dispatch right cluster: %v", err)
	}
	if rightDecision.Request.StartLBA != 116 || rightDecision.Request.Sectors != 4 {
		t.Fatalf("unexpected right-edge cluster request: %+v", rightDecision.Request)
	}
}

func TestResolveTrimPassSuccessesReduceRemainingInterior(t *testing.T) {
	state := mustStartTrimPass(t, mapfile.Extent{
		StartLBA:   100,
		Sectors:    20,
		State:      mapfile.SectorStateSkipped,
		Confidence: mapfile.ConfidenceNone,
	}, 4, 1)

	active, leftCluster, err := DispatchTrimPass(state, true)
	if err != nil {
		t.Fatalf("dispatch left cluster: %v", err)
	}
	leftAdvanced, err := ResolveTrimPass(active, TrimPassOutcome{Request: leftCluster.Request, Success: true})
	if err != nil {
		t.Fatalf("resolve left cluster: %v", err)
	}
	if leftAdvanced.LeftCursor != 104 {
		t.Fatalf("unexpected left cursor after success: %d", leftAdvanced.LeftCursor)
	}

	leftActive, leftFail, err := DispatchTrimPass(leftAdvanced, true)
	if err != nil {
		t.Fatalf("dispatch second left cluster: %v", err)
	}
	rightMode, err := ResolveTrimPass(leftActive, TrimPassOutcome{Request: leftFail.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve second left cluster failure: %v", err)
	}
	leftSingleActive, leftSingle, err := DispatchTrimPass(rightMode, true)
	if err != nil {
		t.Fatalf("dispatch left single: %v", err)
	}
	rightClusterState, err := ResolveTrimPass(leftSingleActive, TrimPassOutcome{Request: leftSingle.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve left single failure: %v", err)
	}

	rightActive, rightCluster, err := DispatchTrimPass(rightClusterState, true)
	if err != nil {
		t.Fatalf("dispatch right cluster: %v", err)
	}
	rightAdvanced, err := ResolveTrimPass(rightActive, TrimPassOutcome{Request: rightCluster.Request, Success: true})
	if err != nil {
		t.Fatalf("resolve right cluster: %v", err)
	}

	remaining := RemainingTrimInterior(rightAdvanced)
	if remaining == nil {
		t.Fatal("expected remaining interior extent")
	}
	if remaining.StartLBA != 104 || remaining.Sectors != 12 {
		t.Fatalf("unexpected remaining interior: %+v", remaining)
	}
}

func TestTrimPassDoneWhenInteriorFullyRecovered(t *testing.T) {
	state := mustStartTrimPass(t, mapfile.Extent{
		StartLBA:   100,
		Sectors:    2,
		State:      mapfile.SectorStateSkipped,
		Confidence: mapfile.ConfidenceNone,
	}, 4, 1)

	active, decision, err := DispatchTrimPass(state, true)
	if err != nil {
		t.Fatalf("dispatch trim pass: %v", err)
	}
	done, err := ResolveTrimPass(active, TrimPassOutcome{Request: decision.Request, Success: true})
	if err != nil {
		t.Fatalf("resolve trim pass: %v", err)
	}
	if done.Mode != TrimPassModeDone {
		t.Fatalf("unexpected final mode: %s", done.Mode)
	}
	if RemainingTrimInterior(done) != nil {
		t.Fatal("expected no remaining interior")
	}
}

func mustStartTrimPass(t *testing.T, target mapfile.Extent, smallCluster uint32, failureBudget uint16) TrimPassState {
	t.Helper()

	state, err := StartTrimPass(target, smallCluster, failureBudget)
	if err != nil {
		t.Fatalf("start trim pass: %v", err)
	}
	return state
}
