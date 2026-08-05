package coordinator

import "testing"

func TestHandleStartProbeAndPrepareBootstrapFlow(t *testing.T) {
	snapshot := Snapshot{Job: Job{State: JobStateIdle}}

	identifying, effects, err := Handle(snapshot, StartJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	assertJobState(t, identifying.Job, JobStateIdentifying)
	assertEffects(t, effects, []Effect{{Kind: EffectProbeMedia, JobID: "job-1", State: JobStateIdentifying}})

	preparing, effects, err := Handle(identifying, ProbeCompleted{JobID: "job-1"})
	if err != nil {
		t.Fatalf("probe completed: %v", err)
	}
	assertJobState(t, preparing.Job, JobStatePreparing)
	assertEffects(t, effects, []Effect{{Kind: EffectBootstrapRecovery, JobID: "job-1", State: JobStatePreparing}})

	running, effects, err := Handle(preparing, WriteResultReceived{JobID: "job-1"})
	if err != nil {
		t.Fatalf("write result received: %v", err)
	}
	assertJobState(t, running.Job, JobStateFastPass)
	assertEffects(t, effects, []Effect{{Kind: EffectDispatchRecovery, JobID: "job-1", State: JobStateFastPass}})
}

func TestHandlePauseCheckpointAndResume(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateAdaptivePass},
	}

	pausing, effects, err := Handle(snapshot, PauseJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("pause job: %v", err)
	}
	if pausing.Job.State != JobStatePausing || pausing.Job.ResumeState != JobStateAdaptivePass {
		t.Fatalf("unexpected pausing state: %+v", pausing.Job)
	}
	assertEffects(t, effects, []Effect{{Kind: EffectCheckpoint, JobID: "job-1", State: JobStateAdaptivePass}})

	paused, effects, err := Handle(pausing, JobCheckpointed{JobID: "job-1"})
	if err != nil {
		t.Fatalf("job checkpointed: %v", err)
	}
	if paused.Job.State != JobStatePaused || paused.Job.ResumeState != JobStateAdaptivePass {
		t.Fatalf("unexpected paused state: %+v", paused.Job)
	}
	assertEffects(t, effects, []Effect{{Kind: EffectPublishPause, JobID: "job-1", State: JobStatePaused}})

	resumed, effects, err := Handle(paused, ResumeJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("resume job: %v", err)
	}
	assertJobState(t, resumed.Job, JobStateAdaptivePass)
	assertEffects(t, effects, []Effect{{Kind: EffectDispatchRecovery, JobID: "job-1", State: JobStateAdaptivePass}})
}

func TestHandleCancelTransitionsToIncompleteAfterCheckpoint(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateScrapePass},
	}

	stopping, effects, err := Handle(snapshot, CancelJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("cancel job: %v", err)
	}
	if stopping.Job.State != JobStateStopping || stopping.Job.ResumeState != JobStateScrapePass {
		t.Fatalf("unexpected stopping state: %+v", stopping.Job)
	}
	assertEffects(t, effects, []Effect{{Kind: EffectCheckpoint, JobID: "job-1", State: JobStateScrapePass}})

	incomplete, effects, err := Handle(stopping, JobCheckpointed{JobID: "job-1"})
	if err != nil {
		t.Fatalf("job checkpointed: %v", err)
	}
	assertJobState(t, incomplete.Job, JobStateIncomplete)
	assertEffects(t, effects, []Effect{{Kind: EffectStopAfterSave, JobID: "job-1", State: JobStateIncomplete}})
}

func TestHandleWorkerResultTransitionsAcrossRecoveryPhases(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateFastPass},
	}

	trim, effects, err := Handle(snapshot, WorkerResultReceived{
		JobID:     "job-1",
		NextState: JobStateTrimPass,
	})
	if err != nil {
		t.Fatalf("worker result received: %v", err)
	}
	assertJobState(t, trim.Job, JobStateTrimPass)
	assertEffects(t, effects, []Effect{{Kind: EffectDispatchRecovery, JobID: "job-1", State: JobStateTrimPass}})
}

func TestHandleVerifyResultCanCompleteJob(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateVerifying},
	}

	completed, effects, err := Handle(snapshot, WorkerResultReceived{
		JobID:     "job-1",
		NextState: JobStateCompleted,
	})
	if err != nil {
		t.Fatalf("worker result received: %v", err)
	}
	assertJobState(t, completed.Job, JobStateCompleted)
	assertEffects(t, effects, nil)
}

func TestHandleFailureMessageMarksJobFailed(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStatePreparing},
	}

	failed, effects, err := Handle(snapshot, JobFailed{JobID: "job-1", Err: errTestFailure})
	if err != nil {
		t.Fatalf("job failed: %v", err)
	}
	assertJobState(t, failed.Job, JobStateFailed)
	assertEffects(t, effects, []Effect{{Kind: EffectReportFailure, JobID: "job-1", State: JobStateFailed}})
}

func TestHandleWorkerUnresponsiveKeepsCoordinatorResponsive(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateFastPass},
	}

	next, effects, err := Handle(snapshot, WorkerUnresponsiveDetected{
		JobID:  "job-1",
		Reason: "hard deadline exceeded",
	})
	if err != nil {
		t.Fatalf("handle worker unresponsive: %v", err)
	}
	assertJobState(t, next.Job, JobStateFastPass)
	assertEffects(t, effects, []Effect{
		{Kind: EffectCheckpoint, JobID: "job-1", State: JobStateFastPass},
		{Kind: EffectWorkerUnresponsive, JobID: "job-1", State: JobStateFastPass},
	})
}

func TestHandleRejectsInvalidWorkerTransition(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateFastPass},
	}

	if _, _, err := Handle(snapshot, WorkerResultReceived{
		JobID:     "job-1",
		NextState: JobStateCompleted,
	}); err == nil {
		t.Fatal("expected invalid fast-pass completion transition to fail")
	}
}

var errTestFailure = testError("test failure")

type testError string

func (e testError) Error() string { return string(e) }

func assertJobState(t *testing.T, job Job, state JobState) {
	t.Helper()
	if job.State != state {
		t.Fatalf("unexpected job state: got %s want %s", job.State, state)
	}
}

func assertEffects(t *testing.T, got, want []Effect) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("unexpected effect count: got %d want %d (effects=%+v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected effect at %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}
