package recovery

import (
	"testing"

	"discrescue/internal/mapfile"
)

func TestDispatchFastPassIssuesClusterReadForUnknownExtent(t *testing.T) {
	state := mustStartFastPass(t, []mapfile.Extent{
		{StartLBA: 0, Sectors: 40, State: mapfile.SectorStateUnknown, Confidence: mapfile.ConfidenceNone},
	}, 32)

	next, decision, err := DispatchFastPass(state, true)
	if err != nil {
		t.Fatalf("dispatch fast pass: %v", err)
	}
	if decision.Kind != FastPassDecisionRead {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if decision.Request.StartLBA != 0 || decision.Request.Sectors != 32 || decision.Request.Strategy != StrategyFast {
		t.Fatalf("unexpected request: %+v", decision.Request)
	}
	if next.ActiveRequest == nil {
		t.Fatal("expected active request to be tracked")
	}
}

func TestDispatchFastPassWaitsForWriterBackpressure(t *testing.T) {
	state := mustStartFastPass(t, []mapfile.Extent{
		{StartLBA: 0, Sectors: 40, State: mapfile.SectorStateUnknown, Confidence: mapfile.ConfidenceNone},
	}, 32)

	next, decision, err := DispatchFastPass(state, false)
	if err != nil {
		t.Fatalf("dispatch fast pass: %v", err)
	}
	if decision.Kind != FastPassDecisionWaitBackpressure {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if next.ActiveRequest != nil {
		t.Fatal("did not expect an active request while backpressured")
	}
}

func TestResolveFastPassClusterFailureSchedulesSingleSectorFallback(t *testing.T) {
	state := mustStartFastPass(t, []mapfile.Extent{
		{StartLBA: 0, Sectors: 40, State: mapfile.SectorStateUnknown, Confidence: mapfile.ConfidenceNone},
	}, 32)

	active, decision, err := DispatchFastPass(state, true)
	if err != nil {
		t.Fatalf("dispatch fast pass: %v", err)
	}
	failed, err := ResolveFastPass(active, FastPassOutcome{Request: decision.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve fast pass: %v", err)
	}
	if failed.Mode != FastPassModeSingle {
		t.Fatalf("unexpected mode after cluster failure: %s", failed.Mode)
	}
	if failed.ClusterBoundary != 32 {
		t.Fatalf("unexpected cluster boundary: %d", failed.ClusterBoundary)
	}

	_, fallback, err := DispatchFastPass(failed, true)
	if err != nil {
		t.Fatalf("dispatch fallback: %v", err)
	}
	if fallback.Request.StartLBA != 0 || fallback.Request.Sectors != 1 {
		t.Fatalf("unexpected fallback request: %+v", fallback.Request)
	}
}

func TestResolveFastPassSingleSuccessContinuesToClusterBoundary(t *testing.T) {
	state := mustStartFastPass(t, []mapfile.Extent{
		{StartLBA: 0, Sectors: 40, State: mapfile.SectorStateUnknown, Confidence: mapfile.ConfidenceNone},
	}, 32)

	active, cluster, err := DispatchFastPass(state, true)
	if err != nil {
		t.Fatalf("dispatch cluster: %v", err)
	}
	singleState, err := ResolveFastPass(active, FastPassOutcome{Request: cluster.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve cluster failure: %v", err)
	}
	active, single, err := DispatchFastPass(singleState, true)
	if err != nil {
		t.Fatalf("dispatch single: %v", err)
	}
	continued, err := ResolveFastPass(active, FastPassOutcome{Request: single.Request, Success: true})
	if err != nil {
		t.Fatalf("resolve single success: %v", err)
	}
	if continued.Mode != FastPassModeSingle {
		t.Fatalf("expected single-sector continuation, got %s", continued.Mode)
	}
	if continued.CursorLBA != 1 {
		t.Fatalf("unexpected cursor after single success: %d", continued.CursorLBA)
	}
}

func TestResolveFastPassSingleFailureMarksFailedSectorAndSkipExtent(t *testing.T) {
	state := mustStartFastPass(t, []mapfile.Extent{
		{StartLBA: 0, Sectors: 400, State: mapfile.SectorStateUnknown, Confidence: mapfile.ConfidenceNone},
	}, 32)

	active, cluster, err := DispatchFastPass(state, true)
	if err != nil {
		t.Fatalf("dispatch cluster: %v", err)
	}
	singleState, err := ResolveFastPass(active, FastPassOutcome{Request: cluster.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve cluster failure: %v", err)
	}
	active, single, err := DispatchFastPass(singleState, true)
	if err != nil {
		t.Fatalf("dispatch single: %v", err)
	}
	failed, err := ResolveFastPass(active, FastPassOutcome{Request: single.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve single failure: %v", err)
	}
	if failed.Mode != FastPassModeCluster {
		t.Fatalf("expected cluster mode after confirmed single-sector failure, got %s", failed.Mode)
	}
	if failed.CursorLBA != 257 {
		t.Fatalf("unexpected cursor after skip advance: %d", failed.CursorLBA)
	}

	ioExtent, _, ok := mapfile.LookupExtent(failed.Extents, 0)
	if !ok || ioExtent.State != mapfile.SectorStateIOError {
		t.Fatalf("expected failed sector to be marked io_error, got %+v", ioExtent)
	}
	skipExtent, _, ok := mapfile.LookupExtent(failed.Extents, 1)
	if !ok || skipExtent.State != mapfile.SectorStateSkipped {
		t.Fatalf("expected skipped extent after failed sector, got %+v", skipExtent)
	}
}

func TestDispatchFastPassEndsWithCheckpointThenDone(t *testing.T) {
	state := mustStartFastPass(t, []mapfile.Extent{
		{StartLBA: 0, Sectors: 8, State: mapfile.SectorStateVerified, Confidence: mapfile.ConfidenceRepeatedSingleCapture},
	}, 32)

	active, checkpoint, err := DispatchFastPass(state, true)
	if err != nil {
		t.Fatalf("dispatch fast pass: %v", err)
	}
	if checkpoint.Kind != FastPassDecisionCheckpoint {
		t.Fatalf("unexpected decision kind: %s", checkpoint.Kind)
	}
	done, err := ResolveFastPass(active, FastPassOutcome{CheckpointCompleted: true})
	if err != nil {
		t.Fatalf("resolve checkpoint: %v", err)
	}
	if done.Mode != FastPassModeDone {
		t.Fatalf("unexpected final mode: %s", done.Mode)
	}
}

func mustStartFastPass(t *testing.T, extents []mapfile.Extent, clusterSize uint32) FastPassState {
	t.Helper()

	state, err := StartFastPass(extents, clusterSize)
	if err != nil {
		t.Fatalf("start fast pass: %v", err)
	}
	return state
}
