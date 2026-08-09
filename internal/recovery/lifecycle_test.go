package recovery

import "testing"

func TestLifecycleStopClosesSchedulingGateAndEscalates(t *testing.T) {
	lifecycle := NewLifecycle()
	if err := lifecycle.Start(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.BeginRead(1); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.RequestStop(StopIntentStop); err != nil {
		t.Fatal(err)
	}
	if lifecycle.State() != JobCancelingRead {
		t.Fatalf("state = %s, want %s", lifecycle.State(), JobCancelingRead)
	}
	if err := lifecycle.BeginRead(2); err == nil {
		t.Fatal("expected scheduling gate to reject a new read")
	}
	if err := lifecycle.GraceExpired(); err != nil {
		t.Fatal(err)
	}
	if !lifecycle.CanForceStop() || lifecycle.State() != JobAwaitingForceStop {
		t.Fatalf("expected force-stop escalation, state=%s canForce=%v", lifecycle.State(), lifecycle.CanForceStop())
	}
	if err := lifecycle.ForceStop(); err != nil {
		t.Fatal(err)
	}
	if lifecycle.State() != JobForceStopping {
		t.Fatalf("state = %s, want %s", lifecycle.State(), JobForceStopping)
	}
}

func TestLifecycleGracefulStopRequiresCheckpointAndRelease(t *testing.T) {
	lifecycle := NewLifecycle()
	if err := lifecycle.Start(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.RequestStop(StopIntentPause); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Checkpointed(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.Released(false); err != nil {
		t.Fatal(err)
	}
	if lifecycle.State() != JobStopped {
		t.Fatalf("state = %s, want %s", lifecycle.State(), JobStopped)
	}
}

func TestLifecycleRejectsInvalidTransitions(t *testing.T) {
	lifecycle := NewLifecycle()
	if err := lifecycle.BeginRead(1); err == nil {
		t.Fatal("expected read before start to fail")
	}
	if err := lifecycle.Start(); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.BeginRead(1); err != nil {
		t.Fatal(err)
	}
	if err := lifecycle.CompleteRead(2); err == nil {
		t.Fatal("expected stale read completion to fail")
	}
	if err := lifecycle.RequestStop(StopIntent("invalid")); err == nil {
		t.Fatal("expected invalid stop intent to fail")
	}
}
