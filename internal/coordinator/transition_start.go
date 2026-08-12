package coordinator

import "fmt"

func calculateStartTransition(snapshot Snapshot, event StartJob) (transition, error) {
	switch snapshot.Job.State {
	case JobStateIdle, JobStateCompleted, JobStateIncomplete, JobStateFailed:
	default:
		return transition{}, fmt.Errorf("start job: invalid state %s", snapshot.Job.State)
	}
	next := Job{ID: event.JobID, State: JobStateIdentifying, NextToken: 1}
	next, effect := issueEffect(next, EffectProbeMedia, JobStateIdentifying)
	return transition{NextJob: next, Effects: []Effect{effect}}, nil
}

func calculateProbeCompletedTransition(snapshot Snapshot, event ProbeCompleted) (transition, error) {
	job, stale, err := acceptPending(snapshot.Job, event.JobID, event.Token, EffectProbeMedia, func(state JobState) bool {
		return state == JobStateIdentifying
	})
	if err != nil {
		return transition{}, err
	}
	if stale {
		return transition{Noop: true}, nil
	}
	if event.Err != nil {
		return buildFailedTransition(job, event.Err), nil
	}
	job.State = JobStatePreparing
	job, effect := issueEffect(job, EffectBootstrapRecovery, JobStatePreparing)
	return transition{NextJob: job, Effects: []Effect{effect}}, nil
}

func calculateWriteResultTransition(snapshot Snapshot, event WriteResultReceived) (transition, error) {
	job, stale, err := acceptPending(snapshot.Job, event.JobID, event.Token, EffectBootstrapRecovery, func(state JobState) bool {
		return state == JobStatePreparing
	})
	if err != nil {
		return transition{}, err
	}
	if stale {
		return transition{Noop: true}, nil
	}
	if event.Err != nil {
		return buildFailedTransition(job, event.Err), nil
	}
	job.State = JobStateFastPass
	job, effect := issueEffect(job, EffectDispatchRecovery, JobStateFastPass)
	return transition{NextJob: job, Effects: []Effect{effect}}, nil
}

func calculateResumeTransition(snapshot Snapshot, event ResumeJob) (transition, error) {
	if err := requireJobState(snapshot.Job, event.JobID, JobStatePaused); err != nil {
		return transition{}, err
	}
	job := snapshot.Job
	job.State = job.ResumeState
	job.ResumeState = ""
	job, effect := issueEffect(job, EffectDispatchRecovery, job.State)
	return transition{NextJob: job, Effects: []Effect{effect}}, nil
}
