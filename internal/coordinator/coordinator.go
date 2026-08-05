package coordinator

import "fmt"

type Snapshot struct {
	Job Job
}

func (s Snapshot) Validate() error {
	return s.Job.Validate()
}

type EffectKind string

const (
	EffectProbeMedia         EffectKind = "probe_media"
	EffectBootstrapRecovery  EffectKind = "bootstrap_recovery"
	EffectDispatchRecovery   EffectKind = "dispatch_recovery"
	EffectCheckpoint         EffectKind = "checkpoint"
	EffectPublishPause       EffectKind = "publish_pause"
	EffectStopAfterSave      EffectKind = "stop_after_save"
	EffectReportFailure      EffectKind = "report_failure"
	EffectWorkerUnresponsive EffectKind = "worker_unresponsive"
)

type Effect struct {
	Kind  EffectKind
	JobID string
	State JobState
}

func Handle(snapshot Snapshot, event Event) (Snapshot, []Effect, error) {
	if err := snapshot.Validate(); err != nil {
		return snapshot, nil, err
	}

	switch typed := event.(type) {
	case StartJob:
		switch snapshot.Job.State {
		case JobStateIdle, JobStateCompleted, JobStateIncomplete, JobStateFailed:
		default:
			return snapshot, nil, fmt.Errorf("start job: invalid state %s", snapshot.Job.State)
		}
		next := snapshot
		next.Job = Job{ID: typed.JobID, State: JobStateIdentifying}
		return next, []Effect{{Kind: EffectProbeMedia, JobID: typed.JobID, State: JobStateIdentifying}}, nil
	case ProbeCompleted:
		if err := requireJobState(snapshot.Job, typed.JobID, JobStateIdentifying); err != nil {
			return snapshot, nil, err
		}
		if typed.Err != nil {
			return failJob(snapshot, typed.JobID, typed.Err)
		}
		next := snapshot
		next.Job.State = JobStatePreparing
		return next, []Effect{{Kind: EffectBootstrapRecovery, JobID: typed.JobID, State: JobStatePreparing}}, nil
	case WriteResultReceived:
		if err := requireJobState(snapshot.Job, typed.JobID, JobStatePreparing); err != nil {
			return snapshot, nil, err
		}
		if typed.Err != nil {
			return failJob(snapshot, typed.JobID, typed.Err)
		}
		next := snapshot
		next.Job.State = JobStateFastPass
		return next, []Effect{{Kind: EffectDispatchRecovery, JobID: typed.JobID, State: JobStateFastPass}}, nil
	case ResumeJob:
		if err := requireJobState(snapshot.Job, typed.JobID, JobStatePaused); err != nil {
			return snapshot, nil, err
		}
		next := snapshot
		next.Job.State = snapshot.Job.ResumeState
		next.Job.ResumeState = ""
		return next, []Effect{{Kind: EffectDispatchRecovery, JobID: typed.JobID, State: next.Job.State}}, nil
	case PauseJob:
		if !runningPhase(snapshot.Job.State) {
			return snapshot, nil, fmt.Errorf("pause job: invalid state %s", snapshot.Job.State)
		}
		next := snapshot
		next.Job.State = JobStatePausing
		next.Job.ResumeState = snapshot.Job.State
		return next, []Effect{{Kind: EffectCheckpoint, JobID: typed.JobID, State: snapshot.Job.State}}, nil
	case CancelJob:
		if snapshot.Job.ID != typed.JobID {
			return snapshot, nil, fmt.Errorf("cancel job: unknown job id %q", typed.JobID)
		}
		next := snapshot
		switch snapshot.Job.State {
		case JobStatePaused:
			next.Job.State = JobStateIncomplete
			next.Job.ResumeState = ""
			return next, []Effect{{Kind: EffectStopAfterSave, JobID: typed.JobID, State: JobStateIncomplete}}, nil
		case JobStatePausing:
			next.Job.State = JobStateStopping
			return next, nil, nil
		default:
			if !runningPhase(snapshot.Job.State) {
				return snapshot, nil, fmt.Errorf("cancel job: invalid state %s", snapshot.Job.State)
			}
			next.Job.State = JobStateStopping
			next.Job.ResumeState = snapshot.Job.State
			return next, []Effect{{Kind: EffectCheckpoint, JobID: typed.JobID, State: snapshot.Job.State}}, nil
		}
	case WorkerResultReceived:
		if snapshot.Job.ID != typed.JobID {
			return snapshot, nil, fmt.Errorf("worker result: unknown job id %q", typed.JobID)
		}
		if !runningPhase(snapshot.Job.State) {
			return snapshot, nil, fmt.Errorf("worker result: invalid state %s", snapshot.Job.State)
		}
		if typed.Err != nil {
			return failJob(snapshot, typed.JobID, typed.Err)
		}
		if !transitionAllowed(snapshot.Job.State, typed.NextState) {
			return snapshot, nil, fmt.Errorf("worker result: invalid transition %s -> %s", snapshot.Job.State, typed.NextState)
		}
		next := snapshot
		next.Job.State = typed.NextState
		effects := []Effect{}
		switch typed.NextState {
		case JobStateCompleted, JobStateIncomplete:
		default:
			effects = append(effects, Effect{Kind: EffectDispatchRecovery, JobID: typed.JobID, State: typed.NextState})
		}
		return next, effects, nil
	case JobCheckpointed:
		if snapshot.Job.ID != typed.JobID {
			return snapshot, nil, fmt.Errorf("job checkpointed: unknown job id %q", typed.JobID)
		}
		next := snapshot
		switch snapshot.Job.State {
		case JobStatePausing:
			next.Job.State = JobStatePaused
			return next, []Effect{{Kind: EffectPublishPause, JobID: typed.JobID, State: JobStatePaused}}, nil
		case JobStateStopping:
			next.Job.State = JobStateIncomplete
			next.Job.ResumeState = ""
			return next, []Effect{{Kind: EffectStopAfterSave, JobID: typed.JobID, State: JobStateIncomplete}}, nil
		default:
			return snapshot, nil, fmt.Errorf("job checkpointed: invalid state %s", snapshot.Job.State)
		}
	case JobFailed:
		if snapshot.Job.ID != typed.JobID {
			return snapshot, nil, fmt.Errorf("job failed: unknown job id %q", typed.JobID)
		}
		return failJob(snapshot, typed.JobID, typed.Err)
	case WorkerUnresponsiveDetected:
		if snapshot.Job.ID != typed.JobID {
			return snapshot, nil, fmt.Errorf("worker unresponsive: unknown job id %q", typed.JobID)
		}
		if !activeJobState(snapshot.Job.State) {
			return snapshot, nil, fmt.Errorf("worker unresponsive: invalid state %s", snapshot.Job.State)
		}
		return snapshot, []Effect{
			{Kind: EffectCheckpoint, JobID: typed.JobID, State: snapshot.Job.State},
			{Kind: EffectWorkerUnresponsive, JobID: typed.JobID, State: snapshot.Job.State},
		}, nil
	default:
		return snapshot, nil, fmt.Errorf("handle event: unsupported %T", event)
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

func failJob(snapshot Snapshot, jobID string, err error) (Snapshot, []Effect, error) {
	if snapshot.Job.ID != jobID {
		return snapshot, nil, fmt.Errorf("fail job: unknown job id %q", jobID)
	}
	if err == nil {
		err = fmt.Errorf("job failed")
	}
	next := snapshot
	next.Job.State = JobStateFailed
	next.Job.ResumeState = ""
	return next, []Effect{{Kind: EffectReportFailure, JobID: jobID, State: JobStateFailed}}, nil
}

func transitionAllowed(current, next JobState) bool {
	switch current {
	case JobStateFastPass:
		return next == JobStateFastPass ||
			next == JobStateTrimPass ||
			next == JobStateAdaptivePass ||
			next == JobStateScrapePass ||
			next == JobStateVerifying
	case JobStateTrimPass:
		return next == JobStateTrimPass ||
			next == JobStateAdaptivePass ||
			next == JobStateScrapePass ||
			next == JobStateVerifying
	case JobStateAdaptivePass:
		return next == JobStateAdaptivePass ||
			next == JobStateScrapePass ||
			next == JobStateVerifying
	case JobStateScrapePass:
		return next == JobStateScrapePass ||
			next == JobStateVerifying
	case JobStateVerifying:
		return next == JobStateCompleted || next == JobStateIncomplete
	default:
		return false
	}
}
