package coordinator

import "fmt"

func acceptPending(job Job, jobID string, token uint64, kind EffectKind, stateAllowed func(JobState) bool) (Job, bool, error) {
	if job.ID != jobID {
		return job, false, fmt.Errorf("pending effect: unknown job id %q", jobID)
	}
	if !stateAllowed(job.State) {
		return job, false, fmt.Errorf("pending effect: invalid state %s", job.State)
	}
	if job.PendingKind != kind || job.PendingToken != token {
		return job, true, nil
	}
	return clearPending(job), false, nil
}

func acceptAnyPending(job Job, jobID string, token uint64, stateAllowed func(JobState) bool) (Job, bool, error) {
	if job.ID != jobID {
		return job, false, fmt.Errorf("pending effect: unknown job id %q", jobID)
	}
	if !stateAllowed(job.State) {
		return job, false, fmt.Errorf("pending effect: invalid state %s", job.State)
	}
	if job.PendingKind == "" || job.PendingToken != token {
		return job, true, nil
	}
	return clearPending(job), false, nil
}
