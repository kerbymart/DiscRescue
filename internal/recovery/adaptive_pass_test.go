package recovery

import (
	"testing"

	"discrescue/internal/mapfile"
)

func TestDispatchAdaptivePassPrioritizesLargestThenFewestAttemptsThenNearestHead(t *testing.T) {
	state, err := StartAdaptivePass([]mapfile.Extent{
		{StartLBA: 100, Sectors: 64, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone, Attempts: 1},
		{StartLBA: 220, Sectors: 64, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone, Attempts: 0},
		{StartLBA: 500, Sectors: 48, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone, Attempts: 2},
	}, 32, 3, 210)
	if err != nil {
		t.Fatalf("start adaptive pass: %v", err)
	}

	_, decision, err := DispatchAdaptivePass(state, true)
	if err != nil {
		t.Fatalf("dispatch adaptive pass: %v", err)
	}
	if decision.Kind != AdaptivePassDecisionRead {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if decision.Request.StartLBA != 220 || decision.Request.Sectors != 32 {
		t.Fatalf("unexpected prioritized request: %+v", decision.Request)
	}
}

func TestDispatchAdaptivePassDefersBelowThresholdToScrape(t *testing.T) {
	state, err := StartAdaptivePass([]mapfile.Extent{
		{StartLBA: 100, Sectors: 16, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 32, 3, 0)
	if err != nil {
		t.Fatalf("start adaptive pass: %v", err)
	}

	active, decision, err := DispatchAdaptivePass(state, true)
	if err != nil {
		t.Fatalf("dispatch adaptive pass: %v", err)
	}
	if decision.Kind != AdaptivePassDecisionDeferToScrape {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if decision.Deferred == nil || decision.Deferred.StartLBA != 100 || decision.Deferred.Sectors != 16 {
		t.Fatalf("unexpected deferred extent: %+v", decision.Deferred)
	}

	resolved, err := ResolveAdaptivePass(active, AdaptivePassOutcome{})
	if err != nil {
		t.Fatalf("resolve adaptive pass: %v", err)
	}
	if len(resolved.DeferredToScrape) != 1 {
		t.Fatalf("expected one deferred extent, got %d", len(resolved.DeferredToScrape))
	}
}

func TestResolveAdaptivePassFailureRequeuesBothSplitHalves(t *testing.T) {
	state, err := StartAdaptivePass([]mapfile.Extent{
		{StartLBA: 100, Sectors: 64, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 32, 3, 0)
	if err != nil {
		t.Fatalf("start adaptive pass: %v", err)
	}

	active, decision, err := DispatchAdaptivePass(state, true)
	if err != nil {
		t.Fatalf("dispatch adaptive pass: %v", err)
	}
	next, err := ResolveAdaptivePass(active, AdaptivePassOutcome{Request: decision.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve adaptive pass: %v", err)
	}
	if len(next.Queue) != 2 {
		t.Fatalf("expected two requeued halves, got %d", len(next.Queue))
	}
	if next.Queue[0].Extent.StartLBA != 100 || next.Queue[0].Extent.Sectors != 32 {
		t.Fatalf("unexpected left half: %+v", next.Queue[0].Extent)
	}
	if next.Queue[1].Extent.StartLBA != 132 || next.Queue[1].Extent.Sectors != 32 {
		t.Fatalf("unexpected right half: %+v", next.Queue[1].Extent)
	}
	if next.Queue[0].Extent.Attempts != 1 || next.Queue[1].Extent.Attempts != 1 {
		t.Fatalf("expected attempts to increment on both halves, got %+v %+v", next.Queue[0].Extent, next.Queue[1].Extent)
	}
}

func TestResolveAdaptivePassSuccessAlternatesProbeDirection(t *testing.T) {
	state, err := StartAdaptivePass([]mapfile.Extent{
		{StartLBA: 100, Sectors: 64, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 32, 3, 0)
	if err != nil {
		t.Fatalf("start adaptive pass: %v", err)
	}

	active, decision, err := DispatchAdaptivePass(state, true)
	if err != nil {
		t.Fatalf("dispatch adaptive pass: %v", err)
	}
	if decision.Direction != AdaptiveProbeLeftFirst || decision.Request.StartLBA != 100 {
		t.Fatalf("unexpected initial probe: direction=%s request=%+v", decision.Direction, decision.Request)
	}

	next, err := ResolveAdaptivePass(active, AdaptivePassOutcome{Request: decision.Request, Success: true})
	if err != nil {
		t.Fatalf("resolve adaptive pass: %v", err)
	}
	if next.NextDirection != AdaptiveProbeRightFirst {
		t.Fatalf("expected next direction to alternate, got %s", next.NextDirection)
	}

	active, second, err := DispatchAdaptivePass(next, true)
	if err != nil {
		t.Fatalf("dispatch second adaptive pass: %v", err)
	}
	if second.Direction != AdaptiveProbeRightFirst {
		t.Fatalf("expected right-first probe after alternation, got %s", second.Direction)
	}
	if second.Request.StartLBA != 116 || second.Request.Sectors != 16 {
		t.Fatalf("unexpected right-first probe request: %+v", second.Request)
	}
	_ = active
}

func TestDispatchAdaptivePassDefersWhenSplitDepthBudgetReached(t *testing.T) {
	state, err := StartAdaptivePass([]mapfile.Extent{
		{StartLBA: 100, Sectors: 64, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 32, 1, 0)
	if err != nil {
		t.Fatalf("start adaptive pass: %v", err)
	}

	active, decision, err := DispatchAdaptivePass(state, true)
	if err != nil {
		t.Fatalf("dispatch adaptive pass: %v", err)
	}
	next, err := ResolveAdaptivePass(active, AdaptivePassOutcome{Request: decision.Request, Success: false})
	if err != nil {
		t.Fatalf("resolve adaptive pass: %v", err)
	}

	_, deferred, err := DispatchAdaptivePass(next, true)
	if err != nil {
		t.Fatalf("dispatch depth-limited adaptive pass: %v", err)
	}
	if deferred.Kind != AdaptivePassDecisionDeferToScrape {
		t.Fatalf("expected defer to scrape after split depth budget, got %s", deferred.Kind)
	}
}

func TestDispatchAdaptivePassWaitsForWriterBackpressure(t *testing.T) {
	state, err := StartAdaptivePass([]mapfile.Extent{
		{StartLBA: 100, Sectors: 64, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 32, 3, 0)
	if err != nil {
		t.Fatalf("start adaptive pass: %v", err)
	}

	next, decision, err := DispatchAdaptivePass(state, false)
	if err != nil {
		t.Fatalf("dispatch adaptive pass: %v", err)
	}
	if decision.Kind != AdaptivePassDecisionWaitBackpressure {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if next.ActiveRequest != nil {
		t.Fatal("did not expect an active request while backpressured")
	}
}
