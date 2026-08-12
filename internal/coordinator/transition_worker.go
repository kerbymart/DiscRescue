package coordinator

import "fmt"

func calculateWorkerResultTransition(snapshot Snapshot, event WorkerResultReceived) (transition, error) {
	job, stale, err := acceptPending(snapshot.Job, event.JobID, event.Token, EffectDispatchRecovery, runningPhase)
	if err != nil {
		return transition{}, err
	}
	if stale {
		return transition{Noop: true}, nil
	}
	if event.Err != nil {
		return buildFailedTransition(job, event.Err), nil
	}
	if !transitionAllowed(snapshot.Job.State, event.NextState) {
		return transition{}, fmt.Errorf("worker result: invalid transition %s -> %s", snapshot.Job.State, event.NextState)
	}
	job.State = event.NextState
	if event.NextState == JobStateCompleted || event.NextState == JobStateIncomplete {
		return transition{NextJob: job}, nil
	}
	job, effect := issueEffect(job, EffectDispatchRecovery, event.NextState)
	return transition{NextJob: job, Effects: []Effect{effect}}, nil
}

func calculateJobFailedTransition(snapshot Snapshot, event JobFailed) (transition, error) {
	job, stale, err := acceptAnyPending(snapshot.Job, event.JobID, event.Token, activeJobState)
	if err != nil {
		return transition{}, err
	}
	if stale {
		return transition{Noop: true}, nil
	}
	return buildFailedTransition(job, event.Err), nil
}

func calculateWorkerUnresponsiveTransition(snapshot Snapshot, event WorkerUnresponsiveDetected) (transition, error) {
	job, stale, err := acceptPending(snapshot.Job, event.JobID, event.Token, EffectDispatchRecovery, activeJobState)
	if err != nil {
		return transition{}, err
	}
	if stale {
		return transition{Noop: true}, nil
	}
	job, checkpoint := issueEffect(job, EffectCheckpoint, snapshot.Job.State)
	return transition{
		NextJob: job,
		Effects: []Effect{
			checkpoint,
			{Kind: EffectWorkerUnresponsive, JobID: event.JobID, State: snapshot.Job.State},
		},
	}, nil
}

func unsupportedEventTransition(event Event) (transition, error) {
	return transition{}, fmt.Errorf("handle event: unsupported %T", event)
}
