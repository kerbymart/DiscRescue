package recovery

import (
	"testing"

	"discrescue/internal/mapfile"
)

func TestDispatchScrapePassIssuesImmediateOneSectorRead(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 2, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 2, 1, 1, 0, true, false)

	next, decision, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch scrape pass: %v", err)
	}
	if decision.Kind != ScrapeDecisionRead {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if decision.Request.StartLBA != 100 || decision.Request.Sectors != 1 || decision.Request.Strategy != StrategyScrape {
		t.Fatalf("unexpected request: %+v", decision.Request)
	}
	if decision.Reverse {
		t.Fatal("did not expect initial immediate read to be reverse")
	}
	if next.ActiveDecision == nil {
		t.Fatal("expected active decision to be tracked")
	}
}

func TestDispatchScrapePassRotatesAcrossUnresolvedSectors(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 2, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 1, 0, 0, 0, false, false)

	active, decision, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch scrape pass: %v", err)
	}
	rotated, err := ResolveScrapePass(active, ScrapePassOutcome{SectorLBA: decision.SectorLBA, Success: false})
	if err != nil {
		t.Fatalf("resolve scrape pass: %v", err)
	}

	_, nextDecision, err := DispatchScrapePass(rotated, true)
	if err != nil {
		t.Fatalf("dispatch rotated scrape pass: %v", err)
	}
	if nextDecision.SectorLBA != 101 {
		t.Fatalf("expected scheduler to rotate to the next sector, got %d", nextDecision.SectorLBA)
	}
}

func TestScrapePassSchedulesDelayBetweenRetriesWhenBudgetAvailable(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 1, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 1, 0, 0, 0, false, false)

	active, decision, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch scrape pass: %v", err)
	}
	retryState, err := ResolveScrapePass(active, ScrapePassOutcome{SectorLBA: decision.SectorLBA, Success: false})
	if err != nil {
		t.Fatalf("resolve scrape pass: %v", err)
	}

	_, delay, err := DispatchScrapePass(retryState, true)
	if err != nil {
		t.Fatalf("dispatch delayed scrape pass: %v", err)
	}
	if delay.Kind != ScrapeDecisionDelay {
		t.Fatalf("expected delay before retry, got %s", delay.Kind)
	}
}

func TestScrapePassEscalatesToReverseTraversalAfterImmediateReads(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 1, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 0, 0, 0, 0, false, false)

	state = failScrapeRead(t, state)
	state = failScrapeRead(t, state)

	_, reverse, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch reverse scrape pass: %v", err)
	}
	if reverse.Kind != ScrapeDecisionRead || !reverse.Reverse {
		t.Fatalf("expected reverse traversal read, got %+v", reverse)
	}
}

func TestScrapePassEscalatesToLowerSpeedWhenSupported(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 1, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 0, 1, 0, 0, true, false)

	state = failScrapeRead(t, state)
	state = failScrapeRead(t, state)
	state = failScrapeRead(t, state)
	state = failScrapeRead(t, state)

	_, speed, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch speed scrape pass: %v", err)
	}
	if speed.Kind != ScrapeDecisionSetSpeed || speed.Speed != ScrapeSpeedMinimum {
		t.Fatalf("expected lower-speed escalation, got %+v", speed)
	}
}

func TestScrapePassEscalatesToReopenAfterLowerSpeedGroup(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 1, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 0, 1, 1, 0, true, false)

	state = exhaustScrapeGroup(t, state, 2)
	state = exhaustScrapeGroup(t, state, 2)

	active, speed, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch speed action: %v", err)
	}
	state, err = ResolveScrapePass(active, ScrapePassOutcome{SectorLBA: speed.SectorLBA})
	if err != nil {
		t.Fatalf("resolve speed action: %v", err)
	}
	state = exhaustScrapeGroup(t, state, 2)

	_, reopen, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch reopen scrape pass: %v", err)
	}
	if reopen.Kind != ScrapeDecisionReopen {
		t.Fatalf("expected reopen escalation, got %+v", reopen)
	}
}

func TestScrapePassDoesNotResetWhenResetDisabledByDefault(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 1, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 0, 0, 1, 1, false, false)

	state = exhaustScrapeGroup(t, state, 2)
	state = exhaustScrapeGroup(t, state, 2)

	active, reopen, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch reopen action: %v", err)
	}
	state, err = ResolveScrapePass(active, ScrapePassOutcome{SectorLBA: reopen.SectorLBA})
	if err != nil {
		t.Fatalf("resolve reopen action: %v", err)
	}
	state = exhaustScrapeGroup(t, state, 1)

	if len(state.FailedSectors) != 1 || state.FailedSectors[0] != 100 {
		t.Fatalf("expected sector to fail without reset escalation, got %+v", state.FailedSectors)
	}
}

func TestScrapePassCanUseOptionalResetWhenEnabled(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 1, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 0, 0, 1, 1, false, true)

	state = exhaustScrapeGroup(t, state, 2)
	state = exhaustScrapeGroup(t, state, 2)

	active, reopen, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch reopen action: %v", err)
	}
	state, err = ResolveScrapePass(active, ScrapePassOutcome{SectorLBA: reopen.SectorLBA})
	if err != nil {
		t.Fatalf("resolve reopen action: %v", err)
	}
	state = exhaustScrapeGroup(t, state, 1)

	_, reset, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch reset action: %v", err)
	}
	if reset.Kind != ScrapeDecisionReset {
		t.Fatalf("expected reset escalation, got %+v", reset)
	}
}

func TestDispatchScrapePassWaitsForWriterBackpressure(t *testing.T) {
	state := mustStartScrapePass(t, []mapfile.Extent{
		{StartLBA: 100, Sectors: 1, State: mapfile.SectorStateSkipped, Confidence: mapfile.ConfidenceNone},
	}, 0, 0, 0, 0, false, false)

	next, decision, err := DispatchScrapePass(state, false)
	if err != nil {
		t.Fatalf("dispatch scrape pass: %v", err)
	}
	if decision.Kind != ScrapeDecisionWaitBackpressure {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
	if next.ActiveDecision != nil {
		t.Fatal("did not expect an active decision while backpressured")
	}
}

func TestDispatchScrapePassDoneWhenQueueEmpty(t *testing.T) {
	state, err := StartScrapePass(nil, 0, 0, 0, 0, false, false)
	if err != nil {
		t.Fatalf("start scrape pass: %v", err)
	}

	_, decision, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch scrape pass: %v", err)
	}
	if decision.Kind != ScrapeDecisionDone {
		t.Fatalf("unexpected decision kind: %s", decision.Kind)
	}
}

func failScrapeRead(t *testing.T, state ScrapePassState) ScrapePassState {
	t.Helper()

	active, decision, err := DispatchScrapePass(state, true)
	if err != nil {
		t.Fatalf("dispatch scrape read: %v", err)
	}
	if decision.Kind != ScrapeDecisionRead {
		t.Fatalf("expected read decision, got %+v", decision)
	}
	next, err := ResolveScrapePass(active, ScrapePassOutcome{SectorLBA: decision.SectorLBA, Success: false})
	if err != nil {
		t.Fatalf("resolve scrape read: %v", err)
	}
	for {
		active, decision, err = DispatchScrapePass(next, true)
		if err != nil {
			t.Fatalf("dispatch scrape follow-up: %v", err)
		}
		if decision.Kind == ScrapeDecisionDelay {
			next, err = ResolveScrapePass(active, ScrapePassOutcome{SectorLBA: decision.SectorLBA})
			if err != nil {
				t.Fatalf("resolve scrape delay: %v", err)
			}
			continue
		}
		if decision.Kind == ScrapeDecisionRead || decision.Kind == ScrapeDecisionSetSpeed || decision.Kind == ScrapeDecisionReopen || decision.Kind == ScrapeDecisionReset || decision.Kind == ScrapeDecisionDone {
			if decision.Kind != ScrapeDecisionRead {
				return next
			}
			return next
		}
		return next
	}
}

func exhaustScrapeGroup(t *testing.T, state ScrapePassState, reads uint8) ScrapePassState {
	t.Helper()

	current := state
	for i := uint8(0); i < reads; i++ {
		current = failScrapeRead(t, current)
	}
	return current
}

func mustStartScrapePass(t *testing.T, extents []mapfile.Extent, delayBudget uint16, speedBudget uint16, reopenBudget uint16, resetBudget uint16, speedSupported bool, resetEnabled bool) ScrapePassState {
	t.Helper()

	state, err := StartScrapePass(extents, delayBudget, speedBudget, reopenBudget, resetBudget, speedSupported, resetEnabled)
	if err != nil {
		t.Fatalf("start scrape pass: %v", err)
	}
	return state
}
