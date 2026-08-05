package coordinator

import "testing"

func TestHandleWorkerUnresponsiveKeepsCoordinatorResponsive(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateRunning},
	}

	next, effects, err := Handle(snapshot, WorkerUnresponsiveDetected{
		JobID:  "job-1",
		Reason: "hard deadline exceeded",
	})
	if err != nil {
		t.Fatalf("handle worker unresponsive: %v", err)
	}
	if next.Job.State != JobStateRunning {
		t.Fatalf("expected coordinator to remain in running state, got %s", next.Job.State)
	}
	if len(effects) != 2 {
		t.Fatalf("expected two effects, got %d", len(effects))
	}
	if effects[0].Kind != "checkpoint" || effects[1].Kind != "worker_unresponsive" {
		t.Fatalf("unexpected effects: %+v", effects)
	}
}
