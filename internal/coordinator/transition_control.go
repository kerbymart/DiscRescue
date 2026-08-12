package coordinator

import "fmt"

func calculatePauseTransition(snapshot Snapshot, event PauseJob) (transition, error) {
	if snapshot.Job.ID != event.JobID {
		return transition{}, fmt.Errorf("pause job: unknown job id %q", event.JobID)
	}
	if !runningPhase(snapshot.Job.State) {
		return transition{}, fmt.Errorf("pause job: invalid state %s", snapshot.Job.State)
	}
	job := clearPending(snapshot.Job)
	job.State = JobStatePausing
	job.ResumeState = snapshot.Job.State
	job, effect := issueEffect(job, EffectCheckpoint, snapshot.Job.State)
	return transition{NextJob: job, Effects: []Effect{effect}}, nil
}

func calculateCancelTransition(snapshot Snapshot, event CancelJob) (transition, error) {
	if snapshot.Job.ID != event.JobID {
		return transition{}, fmt.Errorf("cancel job: unknown job id %q", event.JobID)
	}
	job := snapshot.Job
	switch snapshot.Job.State {
	case JobStatePaused:
		job.State = JobStateIncomplete
		job.ResumeState = ""
		job = clearPending(job)
		return transition{NextJob: job, Effects: []Effect{{Kind: EffectStopAfterSave, JobID: event.JobID, State: JobStateIncomplete}}}, nil
	case JobStatePausing:
		job.State = JobStateStopping
		return transition{NextJob: job}, nil
	default:
		if !runningPhase(snapshot.Job.State) {
			return transition{}, fmt.Errorf("cancel job: invalid state %s", snapshot.Job.State)
		}
		job = clearPending(job)
		job.State = JobStateStopping
		job.ResumeState = snapshot.Job.State
		job, effect := issueEffect(job, EffectCheckpoint, snapshot.Job.State)
		return transition{NextJob: job, Effects: []Effect{effect}}, nil
	}
}

func calculateCheckpointTransition(snapshot Snapshot, event JobCheckpointed) (transition, error) {
	job, stale, err := acceptPending(snapshot.Job, event.JobID, event.Token, EffectCheckpoint, func(state JobState) bool {
		return state == JobStatePausing || state == JobStateStopping
	})
	if err != nil {
		return transition{}, err
	}
	if stale {
		return transition{Noop: true}, nil
	}
	switch snapshot.Job.State {
	case JobStatePausing:
		job.State = JobStatePaused
		return transition{NextJob: job, Effects: []Effect{{Kind: EffectPublishPause, JobID: event.JobID, State: JobStatePaused}}}, nil
	case JobStateStopping:
		job.State = JobStateIncomplete
		job.ResumeState = ""
		return transition{NextJob: job, Effects: []Effect{{Kind: EffectStopAfterSave, JobID: event.JobID, State: JobStateIncomplete}}}, nil
	default:
		return transition{}, fmt.Errorf("job checkpointed: invalid state %s", snapshot.Job.State)
	}
}
