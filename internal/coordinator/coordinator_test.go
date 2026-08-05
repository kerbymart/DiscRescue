package coordinator

import "testing"

func TestHandleStartProbeAndPrepareBootstrapFlow(t *testing.T) {
	snapshot := Snapshot{Job: Job{State: JobStateIdle}}

	identifying, effects, err := Handle(snapshot, StartJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	assertJobState(t, identifying.Job, JobStateIdentifying)
	assertEffects(t, effects, []Effect{{Kind: EffectProbeMedia, JobID: "job-1", State: JobStateIdentifying, Token: 1}})

	preparing, effects, err := Handle(identifying, ProbeCompleted{JobID: "job-1", Token: 1})
	if err != nil {
		t.Fatalf("probe completed: %v", err)
	}
	assertJobState(t, preparing.Job, JobStatePreparing)
	assertEffects(t, effects, []Effect{{Kind: EffectBootstrapRecovery, JobID: "job-1", State: JobStatePreparing, Token: 2}})

	running, effects, err := Handle(preparing, WriteResultReceived{JobID: "job-1", Token: 2})
	if err != nil {
		t.Fatalf("write result received: %v", err)
	}
	assertJobState(t, running.Job, JobStateFastPass)
	assertEffects(t, effects, []Effect{{Kind: EffectDispatchRecovery, JobID: "job-1", State: JobStateFastPass, Token: 3}})
}

func TestHandlePauseCheckpointAndResume(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateAdaptivePass, PendingKind: EffectDispatchRecovery, PendingToken: 1, NextToken: 2},
	}

	pausing, effects, err := Handle(snapshot, PauseJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("pause job: %v", err)
	}
	if pausing.Job.State != JobStatePausing || pausing.Job.ResumeState != JobStateAdaptivePass {
		t.Fatalf("unexpected pausing state: %+v", pausing.Job)
	}
	assertEffects(t, effects, []Effect{{Kind: EffectCheckpoint, JobID: "job-1", State: JobStateAdaptivePass, Token: 2}})

	paused, effects, err := Handle(pausing, JobCheckpointed{JobID: "job-1", Token: 2})
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
	assertEffects(t, effects, []Effect{{Kind: EffectDispatchRecovery, JobID: "job-1", State: JobStateAdaptivePass, Token: 3}})
}

func TestHandleCancelTransitionsToIncompleteAfterCheckpoint(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateScrapePass, PendingKind: EffectDispatchRecovery, PendingToken: 1, NextToken: 2},
	}

	stopping, effects, err := Handle(snapshot, CancelJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("cancel job: %v", err)
	}
	if stopping.Job.State != JobStateStopping || stopping.Job.ResumeState != JobStateScrapePass {
		t.Fatalf("unexpected stopping state: %+v", stopping.Job)
	}
	assertEffects(t, effects, []Effect{{Kind: EffectCheckpoint, JobID: "job-1", State: JobStateScrapePass, Token: 2}})

	incomplete, effects, err := Handle(stopping, JobCheckpointed{JobID: "job-1", Token: 2})
	if err != nil {
		t.Fatalf("job checkpointed: %v", err)
	}
	assertJobState(t, incomplete.Job, JobStateIncomplete)
	assertEffects(t, effects, []Effect{{Kind: EffectStopAfterSave, JobID: "job-1", State: JobStateIncomplete}})
}

func TestHandleWorkerResultTransitionsAcrossRecoveryPhases(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateFastPass, PendingKind: EffectDispatchRecovery, PendingToken: 1, NextToken: 2},
	}

	trim, effects, err := Handle(snapshot, WorkerResultReceived{
		JobID:     "job-1",
		Token:     1,
		NextState: JobStateTrimPass,
	})
	if err != nil {
		t.Fatalf("worker result received: %v", err)
	}
	assertJobState(t, trim.Job, JobStateTrimPass)
	assertEffects(t, effects, []Effect{{Kind: EffectDispatchRecovery, JobID: "job-1", State: JobStateTrimPass, Token: 2}})
}

func TestHandleVerifyResultCanCompleteJob(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateVerifying, PendingKind: EffectDispatchRecovery, PendingToken: 1, NextToken: 2},
	}

	completed, effects, err := Handle(snapshot, WorkerResultReceived{
		JobID:     "job-1",
		Token:     1,
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
		Job: Job{ID: "job-1", State: JobStatePreparing, PendingKind: EffectBootstrapRecovery, PendingToken: 1, NextToken: 2},
	}

	failed, effects, err := Handle(snapshot, JobFailed{JobID: "job-1", Token: 1, Err: errTestFailure})
	if err != nil {
		t.Fatalf("job failed: %v", err)
	}
	assertJobState(t, failed.Job, JobStateFailed)
	assertEffects(t, effects, []Effect{{Kind: EffectReportFailure, JobID: "job-1", State: JobStateFailed}})
}

func TestHandleWorkerUnresponsiveKeepsCoordinatorResponsive(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateFastPass, PendingKind: EffectDispatchRecovery, PendingToken: 1, NextToken: 2},
	}

	next, effects, err := Handle(snapshot, WorkerUnresponsiveDetected{
		JobID:  "job-1",
		Token:  1,
		Reason: "hard deadline exceeded",
	})
	if err != nil {
		t.Fatalf("handle worker unresponsive: %v", err)
	}
	assertJobState(t, next.Job, JobStateFastPass)
	assertEffects(t, effects, []Effect{
		{Kind: EffectCheckpoint, JobID: "job-1", State: JobStateFastPass, Token: 2},
		{Kind: EffectWorkerUnresponsive, JobID: "job-1", State: JobStateFastPass},
	})
}

func TestHandleRejectsInvalidWorkerTransition(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateFastPass, PendingKind: EffectDispatchRecovery, PendingToken: 1, NextToken: 2},
	}

	if _, _, err := Handle(snapshot, WorkerResultReceived{
		JobID:     "job-1",
		Token:     1,
		NextState: JobStateCompleted,
	}); err == nil {
		t.Fatal("expected invalid fast-pass completion transition to fail")
	}
}

func TestHandleIgnoresStaleProbeCompletion(t *testing.T) {
	snapshot, _, err := Handle(Snapshot{Job: Job{State: JobStateIdle}}, StartJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}

	next, effects, err := Handle(snapshot, ProbeCompleted{JobID: "job-1", Token: 99})
	if err != nil {
		t.Fatalf("stale probe completed: %v", err)
	}
	if next != snapshot {
		t.Fatalf("expected stale probe result to leave snapshot unchanged, got %+v", next)
	}
	assertEffects(t, effects, nil)
}

func TestHandleIgnoresStaleWorkerResultAfterResume(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateAdaptivePass, NextToken: 2, PendingKind: EffectDispatchRecovery, PendingToken: 1},
	}

	pausing, _, err := Handle(snapshot, PauseJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("pause job: %v", err)
	}
	paused, _, err := Handle(pausing, JobCheckpointed{JobID: "job-1", Token: 2})
	if err != nil {
		t.Fatalf("job checkpointed: %v", err)
	}
	resumed, _, err := Handle(paused, ResumeJob{JobID: "job-1"})
	if err != nil {
		t.Fatalf("resume job: %v", err)
	}

	next, effects, err := Handle(resumed, WorkerResultReceived{
		JobID:     "job-1",
		Token:     1,
		NextState: JobStateScrapePass,
	})
	if err != nil {
		t.Fatalf("stale worker result: %v", err)
	}
	if next != resumed {
		t.Fatalf("expected stale worker result to leave snapshot unchanged, got %+v", next)
	}
	assertEffects(t, effects, nil)
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
