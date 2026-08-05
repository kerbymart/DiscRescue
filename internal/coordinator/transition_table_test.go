package coordinator

import (
	"strings"
	"testing"
)

func TestHandleTransitionTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		start           Snapshot
		event           Event
		wantState       JobState
		wantResumeState JobState
		wantEffects     []Effect
		wantNoChange    bool
		wantErrContains string
	}{
		{
			name:            "duplicate start while identifying is rejected",
			start:           snapshotAfterStart(t, "job-1"),
			event:           StartJob{JobID: "job-2"},
			wantState:       JobStateIdentifying,
			wantErrContains: "start job: invalid state identifying",
		},
		{
			name: "duplicate pause while already pausing is rejected",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStatePausing, ResumeState: JobStateFastPass, PendingKind: EffectCheckpoint, PendingToken: 2, NextToken: 3},
			},
			event:           PauseJob{JobID: "job-1"},
			wantState:       JobStatePausing,
			wantResumeState: JobStateFastPass,
			wantErrContains: "pause job: invalid state pausing",
		},
		{
			name: "checkpoint in running state is rejected",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStateFastPass, PendingKind: EffectDispatchRecovery, PendingToken: 3, NextToken: 4},
			},
			event:           JobCheckpointed{JobID: "job-1", Token: 3},
			wantState:       JobStateFastPass,
			wantErrContains: "pending effect: invalid state fast_pass",
		},
		{
			name: "cancel from paused completes as incomplete without checkpoint",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStatePaused, ResumeState: JobStateTrimPass, NextToken: 3},
			},
			event:           CancelJob{JobID: "job-1"},
			wantState:       JobStateIncomplete,
			wantResumeState: "",
			wantEffects: []Effect{
				{Kind: EffectStopAfterSave, JobID: "job-1", State: JobStateIncomplete},
			},
		},
		{
			name: "cancel while pausing transitions to stopping without new effects",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStatePausing, ResumeState: JobStateAdaptivePass, PendingKind: EffectCheckpoint, PendingToken: 4, NextToken: 5},
			},
			event:           CancelJob{JobID: "job-1"},
			wantState:       JobStateStopping,
			wantResumeState: JobStateAdaptivePass,
			wantEffects:     nil,
		},
		{
			name: "bootstrap write failure marks failed",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStatePreparing, PendingKind: EffectBootstrapRecovery, PendingToken: 2, NextToken: 3},
			},
			event:     WriteResultReceived{JobID: "job-1", Token: 2, Err: errTestFailure},
			wantState: JobStateFailed,
			wantEffects: []Effect{
				{Kind: EffectReportFailure, JobID: "job-1", State: JobStateFailed},
			},
		},
		{
			name: "worker result failure marks failed",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStateScrapePass, PendingKind: EffectDispatchRecovery, PendingToken: 7, NextToken: 8},
			},
			event:     WorkerResultReceived{JobID: "job-1", Token: 7, Err: errTestFailure},
			wantState: JobStateFailed,
			wantEffects: []Effect{
				{Kind: EffectReportFailure, JobID: "job-1", State: JobStateFailed},
			},
		},
		{
			name: "worker crash failure on checkpoint pending marks failed",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStateStopping, ResumeState: JobStateScrapePass, PendingKind: EffectCheckpoint, PendingToken: 5, NextToken: 6},
			},
			event:           JobFailed{JobID: "job-1", Token: 5, Err: errTestFailure},
			wantState:       JobStateFailed,
			wantResumeState: "",
			wantEffects: []Effect{
				{Kind: EffectReportFailure, JobID: "job-1", State: JobStateFailed},
			},
		},
		{
			name: "duplicate checkpoint after paused state is rejected as invalid",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStatePaused, ResumeState: JobStateFastPass, NextToken: 3},
			},
			event:           JobCheckpointed{JobID: "job-1", Token: 2},
			wantState:       JobStatePaused,
			wantResumeState: JobStateFastPass,
			wantErrContains: "pending effect: invalid state paused",
		},
		{
			name: "duplicate failure after failed state is rejected as invalid",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStateFailed, NextToken: 4},
			},
			event:           JobFailed{JobID: "job-1", Token: 3, Err: errTestFailure},
			wantState:       JobStateFailed,
			wantErrContains: "pending effect: invalid state failed",
		},
		{
			name: "worker crash alert from verifying issues checkpoint and alert",
			start: Snapshot{
				Job: Job{ID: "job-1", State: JobStateVerifying, PendingKind: EffectDispatchRecovery, PendingToken: 8, NextToken: 9},
			},
			event:     WorkerUnresponsiveDetected{JobID: "job-1", Token: 8, Reason: "worker exited"},
			wantState: JobStateVerifying,
			wantEffects: []Effect{
				{Kind: EffectCheckpoint, JobID: "job-1", State: JobStateVerifying, Token: 9},
				{Kind: EffectWorkerUnresponsive, JobID: "job-1", State: JobStateVerifying},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next, effects, err := Handle(tt.start, tt.event)

			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErrContains)
				}
				if !strings.Contains(err.Error(), tt.wantErrContains) {
					t.Fatalf("unexpected error: got %q want substring %q", err.Error(), tt.wantErrContains)
				}
				if next != tt.start {
					t.Fatalf("expected state to remain unchanged on error, got %+v", next)
				}
				assertEffects(t, effects, nil)
				return
			}

			if err != nil {
				t.Fatalf("handle event: %v", err)
			}
			if tt.wantNoChange {
				if next != tt.start {
					t.Fatalf("expected no change, got %+v want %+v", next, tt.start)
				}
				assertEffects(t, effects, nil)
				return
			}

			assertJobState(t, next.Job, tt.wantState)
			if next.Job.ResumeState != tt.wantResumeState {
				t.Fatalf("unexpected resume state: got %s want %s", next.Job.ResumeState, tt.wantResumeState)
			}
			assertEffects(t, effects, tt.wantEffects)
		})
	}
}

func TestHandleWorkerCrashSequenceFromRunningToFailed(t *testing.T) {
	snapshot := Snapshot{
		Job: Job{ID: "job-1", State: JobStateAdaptivePass, PendingKind: EffectDispatchRecovery, PendingToken: 3, NextToken: 4},
	}

	next, effects, err := Handle(snapshot, WorkerUnresponsiveDetected{JobID: "job-1", Token: 3, Reason: "worker exited"})
	if err != nil {
		t.Fatalf("worker unresponsive: %v", err)
	}
	assertJobState(t, next.Job, JobStateAdaptivePass)
	assertEffects(t, effects, []Effect{
		{Kind: EffectCheckpoint, JobID: "job-1", State: JobStateAdaptivePass, Token: 4},
		{Kind: EffectWorkerUnresponsive, JobID: "job-1", State: JobStateAdaptivePass},
	})

	failed, effects, err := Handle(next, JobFailed{JobID: "job-1", Token: 4, Err: errTestFailure})
	if err != nil {
		t.Fatalf("job failed: %v", err)
	}
	assertJobState(t, failed.Job, JobStateFailed)
	assertEffects(t, effects, []Effect{
		{Kind: EffectReportFailure, JobID: "job-1", State: JobStateFailed},
	})
}

func snapshotAfterStart(t *testing.T, jobID string) Snapshot {
	t.Helper()
	next, _, err := Handle(Snapshot{Job: Job{State: JobStateIdle}}, StartJob{JobID: jobID})
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	return next
}
