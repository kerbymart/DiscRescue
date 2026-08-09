package recovery

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestStopControllerEscalatesBlockedReadAndReleasesAfterDurability(t *testing.T) {
	var canceled, forced, checkpointed, released atomic.Int32
	controller, err := NewStopController(5*time.Millisecond, StopHooks{
		CancelRead:    func() error { canceled.Add(1); return nil },
		ForceStop:     func() error { forced.Add(1); return nil },
		Checkpoint:    func() error { checkpointed.Add(1); return nil },
		ReleaseDevice: func() error { released.Add(1); return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	if err := controller.BeginRead(1); err != nil {
		t.Fatal(err)
	}
	if err := controller.RequestStop(StopIntentStop); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	state, canForce := controller.Snapshot()
	if state != JobAwaitingForceStop || !canForce {
		t.Fatalf("state=%s canForce=%v, want awaiting force stop", state, canForce)
	}
	if err := controller.ForceStop(); err != nil {
		t.Fatal(err)
	}
	if err := controller.ReadFinished(1); err != nil {
		t.Fatal(err)
	}
	state, _ = controller.Snapshot()
	if state != JobStopped {
		t.Fatalf("state=%s, want stopped", state)
	}
	if canceled.Load() != 1 || forced.Load() != 1 || checkpointed.Load() != 1 || released.Load() != 1 {
		t.Fatalf("hooks canceled=%d forced=%d checkpointed=%d released=%d", canceled.Load(), forced.Load(), checkpointed.Load(), released.Load())
	}
}

func TestStopControllerDoesNotClaimStoppedWhenCheckpointFails(t *testing.T) {
	controller, err := NewStopController(time.Second, StopHooks{
		Checkpoint:    func() error { return errors.New("disk full") },
		ReleaseDevice: func() error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	if err := controller.RequestStop(StopIntentPause); err == nil {
		t.Fatal("expected checkpoint failure")
	}
	state, _ := controller.Snapshot()
	if state != JobFailed {
		t.Fatalf("state=%s, want failed", state)
	}
}
