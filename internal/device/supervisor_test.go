package device

import (
	"testing"
	"time"
)

func TestSupervisorAcquireAndReleaseSingleDrive(t *testing.T) {
	supervisor := Supervisor{}

	next, ok := supervisor.AcquireDrive("/dev/sr0")
	if !ok {
		t.Fatal("expected first drive acquire to succeed")
	}
	if !next.CanOwnDrive("/dev/sr0") {
		t.Fatal("expected supervisor to own the acquired drive")
	}

	if _, ok := next.AcquireDrive("/dev/sr1"); ok {
		t.Fatal("expected second distinct drive acquire to fail")
	}

	released := next.ReleaseDrive("/dev/sr0")
	if !released.CanOwnDrive("/dev/sr1") {
		t.Fatal("expected released supervisor to allow a different drive")
	}
}

func TestSupervisorDispatchAssignsMonotonicRequestID(t *testing.T) {
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	supervisor := Supervisor{}
	deadlines := Deadlines{Soft: 15 * time.Second, Hard: 45 * time.Second, RetryBudget: 3}

	next, effects, err := supervisor.Dispatch(now, "/dev/sr0", CommandRequest{Command: CommandReadBlocks, Sectors: 16}, deadlines)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(effects) != 1 || effects[0].Kind != EffectDispatchRequest {
		t.Fatalf("unexpected dispatch effects: %+v", effects)
	}
	if next.Active == nil || next.Active.Request.ID != 1 {
		t.Fatalf("expected active request id 1, got %+v", next.Active)
	}
	if next.NextRequestID != 1 {
		t.Fatalf("expected next request id to advance, got %d", next.NextRequestID)
	}
}

func TestSupervisorDispatchRejectsSecondActiveCommand(t *testing.T) {
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	supervisor := Supervisor{}
	deadlines := Deadlines{Soft: 10 * time.Second, Hard: 30 * time.Second, RetryBudget: 1}

	active, _, err := supervisor.Dispatch(now, "/dev/sr0", CommandRequest{Command: CommandInquiry}, deadlines)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if _, _, err := active.Dispatch(now, "/dev/sr0", CommandRequest{Command: CommandInquiry}, deadlines); err == nil {
		t.Fatal("expected second active command dispatch to fail")
	}
}

func TestSupervisorObserveResultRejectsStaleResult(t *testing.T) {
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	supervisor := Supervisor{}
	deadlines := Deadlines{Soft: 10 * time.Second, Hard: 30 * time.Second, RetryBudget: 1}

	active, _, err := supervisor.Dispatch(now, "/dev/sr0", CommandRequest{Command: CommandInquiry}, deadlines)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	unchanged, stale, err := active.ObserveResult(999)
	if err != nil {
		t.Fatalf("observe result: %v", err)
	}
	if !stale {
		t.Fatal("expected mismatched request id to be stale")
	}
	if unchanged.Active == nil || unchanged.Active.Request.ID != active.Active.Request.ID {
		t.Fatalf("expected active request to remain unchanged, got %+v", unchanged.Active)
	}
}

func TestSupervisorCheckDeadlinesEmitsSoftThenHard(t *testing.T) {
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	supervisor := Supervisor{}
	deadlines := Deadlines{Soft: 5 * time.Second, Hard: 15 * time.Second, RetryBudget: 1}

	active, _, err := supervisor.Dispatch(now, "/dev/sr0", CommandRequest{Command: CommandSetSpeed}, deadlines)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	afterSoft, effects, err := active.CheckDeadlines(now.Add(5 * time.Second))
	if err != nil {
		t.Fatalf("check deadlines: %v", err)
	}
	if len(effects) != 1 || effects[0].Kind != EffectSoftDeadline {
		t.Fatalf("expected soft deadline effect, got %+v", effects)
	}

	_, effects, err = afterSoft.CheckDeadlines(now.Add(15 * time.Second))
	if err != nil {
		t.Fatalf("check deadlines: %v", err)
	}
	if len(effects) == 0 || effects[len(effects)-1].Kind != EffectHardDeadline {
		t.Fatalf("expected hard deadline effect, got %+v", effects)
	}
}

func TestSupervisorRestartDecisionHonorsPolicy(t *testing.T) {
	supervisor := Supervisor{OwnedDrive: "/dev/sr0", RestartPolicy: RestartRetryable}

	if effects := supervisor.RestartDecision(false); len(effects) != 0 {
		t.Fatalf("expected no restart for non-retryable failure, got %+v", effects)
	}
	effects := supervisor.RestartDecision(true)
	if len(effects) != 1 || effects[0].Kind != EffectRestartWorker {
		t.Fatalf("expected restart effect for retryable failure, got %+v", effects)
	}
}
