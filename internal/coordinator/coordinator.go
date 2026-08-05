package coordinator

import "fmt"

type Snapshot struct {
	Job Job
}

type Effect struct {
	Kind  string
	JobID string
}

func Handle(snapshot Snapshot, event Event) (Snapshot, []Effect, error) {
	switch typed := event.(type) {
	case StartJob:
		if snapshot.Job.State != JobStateIdle {
			return snapshot, nil, fmt.Errorf("start job: invalid state %s", snapshot.Job.State)
		}
		next := snapshot
		next.Job = Job{ID: typed.JobID, State: JobStateRunning}
		return next, []Effect{{Kind: "start-worker", JobID: typed.JobID}}, nil
	case PauseJob:
		if snapshot.Job.State != JobStateRunning {
			return snapshot, nil, fmt.Errorf("pause job: invalid state %s", snapshot.Job.State)
		}
		next := snapshot
		next.Job.State = JobStatePaused
		return next, []Effect{{Kind: "shutdown_pause", JobID: typed.JobID}}, nil
	case CancelJob:
		next := snapshot
		next.Job.State = JobStateIdle
		return next, []Effect{{Kind: "shutdown_cancel", JobID: typed.JobID}}, nil
	case WorkerUnresponsiveDetected:
		if snapshot.Job.State != JobStateRunning && snapshot.Job.State != JobStatePaused {
			return snapshot, nil, fmt.Errorf("worker unresponsive: invalid state %s", snapshot.Job.State)
		}
		return snapshot, []Effect{
			{Kind: "checkpoint", JobID: typed.JobID},
			{Kind: "worker_unresponsive", JobID: typed.JobID},
		}, nil
	default:
		return snapshot, nil, fmt.Errorf("handle event: unsupported %T", event)
	}
}
