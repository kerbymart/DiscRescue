package coordinator

import "fmt"

type Snapshot struct {
	Job Job
}

func (s Snapshot) Validate() error {
	return s.Job.Validate()
}

func Handle(snapshot Snapshot, event Event) (Snapshot, []Effect, error) {
	if err := snapshot.Validate(); err != nil {
		return snapshot, nil, err
	}

	transition, err := calculateTransition(snapshot, event)
	if err != nil {
		return snapshot, nil, err
	}
	if transition.Noop {
		return snapshot, nil, nil
	}
	if err := validateTransition(transition); err != nil {
		return snapshot, nil, err
	}
	next := commitTransition(snapshot, transition)
	return next, publishEffects(transition), nil
}

func calculateTransition(snapshot Snapshot, event Event) (transition, error) {
	switch typed := event.(type) {
	case StartJob:
		return calculateStartTransition(snapshot, typed)
	case ProbeCompleted:
		return calculateProbeCompletedTransition(snapshot, typed)
	case WriteResultReceived:
		return calculateWriteResultTransition(snapshot, typed)
	case ResumeJob:
		return calculateResumeTransition(snapshot, typed)
	case PauseJob:
		return calculatePauseTransition(snapshot, typed)
	case CancelJob:
		return calculateCancelTransition(snapshot, typed)
	case WorkerResultReceived:
		return calculateWorkerResultTransition(snapshot, typed)
	case JobCheckpointed:
		return calculateCheckpointTransition(snapshot, typed)
	case JobFailed:
		return calculateJobFailedTransition(snapshot, typed)
	case WorkerUnresponsiveDetected:
		return calculateWorkerUnresponsiveTransition(snapshot, typed)
	default:
		return unsupportedEventTransition(event)
	}
}

func requireJobState(job Job, jobID string, state JobState) error {
	if job.ID != jobID {
		return fmt.Errorf("job state: unknown job id %q", jobID)
	}
	if job.State != state {
		return fmt.Errorf("job state: expected %s, got %s", state, job.State)
	}
	return nil
}
