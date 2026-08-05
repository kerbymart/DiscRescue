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
	Token uint64
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

type transition struct {
	NextJob Job
	Effects []Effect
	Noop    bool
}

func calculateTransition(snapshot Snapshot, event Event) (transition, error) {
	switch typed := event.(type) {
	case StartJob:
		switch snapshot.Job.State {
		case JobStateIdle, JobStateCompleted, JobStateIncomplete, JobStateFailed:
		default:
			return transition{}, fmt.Errorf("start job: invalid state %s", snapshot.Job.State)
		}
		next := Job{
			ID:        typed.JobID,
			State:     JobStateIdentifying,
			NextToken: 1,
		}
		next, effect := issueEffect(next, EffectProbeMedia, JobStateIdentifying)
		return transition{NextJob: next, Effects: []Effect{effect}}, nil
	case ProbeCompleted:
		job, stale, err := acceptPending(snapshot.Job, typed.JobID, typed.Token, EffectProbeMedia, func(state JobState) bool {
			return state == JobStateIdentifying
		})
		if err != nil {
			return transition{}, err
		}
		if stale {
			return transition{Noop: true}, nil
		}
		if typed.Err != nil {
			return buildFailedTransition(job, typed.Err), nil
		}
		job.State = JobStatePreparing
		job, effect := issueEffect(job, EffectBootstrapRecovery, JobStatePreparing)
		return transition{NextJob: job, Effects: []Effect{effect}}, nil
	case WriteResultReceived:
		job, stale, err := acceptPending(snapshot.Job, typed.JobID, typed.Token, EffectBootstrapRecovery, func(state JobState) bool {
			return state == JobStatePreparing
		})
		if err != nil {
			return transition{}, err
		}
		if stale {
			return transition{Noop: true}, nil
		}
		if typed.Err != nil {
			return buildFailedTransition(job, typed.Err), nil
		}
		job.State = JobStateFastPass
		job, effect := issueEffect(job, EffectDispatchRecovery, JobStateFastPass)
		return transition{NextJob: job, Effects: []Effect{effect}}, nil
	case ResumeJob:
		if err := requireJobState(snapshot.Job, typed.JobID, JobStatePaused); err != nil {
			return transition{}, err
		}
		job := snapshot.Job
		job.State = job.ResumeState
		job.ResumeState = ""
		job, effect := issueEffect(job, EffectDispatchRecovery, job.State)
		return transition{NextJob: job, Effects: []Effect{effect}}, nil
	case PauseJob:
		if snapshot.Job.ID != typed.JobID {
			return transition{}, fmt.Errorf("pause job: unknown job id %q", typed.JobID)
		}
		if !runningPhase(snapshot.Job.State) {
			return transition{}, fmt.Errorf("pause job: invalid state %s", snapshot.Job.State)
		}
		job := clearPending(snapshot.Job)
		job.State = JobStatePausing
		job.ResumeState = snapshot.Job.State
		job, effect := issueEffect(job, EffectCheckpoint, snapshot.Job.State)
		return transition{NextJob: job, Effects: []Effect{effect}}, nil
	case CancelJob:
		if snapshot.Job.ID != typed.JobID {
			return transition{}, fmt.Errorf("cancel job: unknown job id %q", typed.JobID)
		}
		job := snapshot.Job
		switch snapshot.Job.State {
		case JobStatePaused:
			job.State = JobStateIncomplete
			job.ResumeState = ""
			job = clearPending(job)
			return transition{NextJob: job, Effects: []Effect{{Kind: EffectStopAfterSave, JobID: typed.JobID, State: JobStateIncomplete}}}, nil
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
	case WorkerResultReceived:
		job, stale, err := acceptPending(snapshot.Job, typed.JobID, typed.Token, EffectDispatchRecovery, runningPhase)
		if err != nil {
			return transition{}, err
		}
		if stale {
			return transition{Noop: true}, nil
		}
		if typed.Err != nil {
			return buildFailedTransition(job, typed.Err), nil
		}
		if !transitionAllowed(snapshot.Job.State, typed.NextState) {
			return transition{}, fmt.Errorf("worker result: invalid transition %s -> %s", snapshot.Job.State, typed.NextState)
		}
		job.State = typed.NextState
		if typed.NextState == JobStateCompleted || typed.NextState == JobStateIncomplete {
			return transition{NextJob: job}, nil
		}
		job, effect := issueEffect(job, EffectDispatchRecovery, typed.NextState)
		return transition{NextJob: job, Effects: []Effect{effect}}, nil
	case JobCheckpointed:
		job, stale, err := acceptPending(snapshot.Job, typed.JobID, typed.Token, EffectCheckpoint, func(state JobState) bool {
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
			return transition{NextJob: job, Effects: []Effect{{Kind: EffectPublishPause, JobID: typed.JobID, State: JobStatePaused}}}, nil
		case JobStateStopping:
			job.State = JobStateIncomplete
			job.ResumeState = ""
			return transition{NextJob: job, Effects: []Effect{{Kind: EffectStopAfterSave, JobID: typed.JobID, State: JobStateIncomplete}}}, nil
		default:
			return transition{}, fmt.Errorf("job checkpointed: invalid state %s", snapshot.Job.State)
		}
	case JobFailed:
		job, stale, err := acceptAnyPending(snapshot.Job, typed.JobID, typed.Token, activeJobState)
		if err != nil {
			return transition{}, err
		}
		if stale {
			return transition{Noop: true}, nil
		}
		return buildFailedTransition(job, typed.Err), nil
	case WorkerUnresponsiveDetected:
		job, stale, err := acceptPending(snapshot.Job, typed.JobID, typed.Token, EffectDispatchRecovery, activeJobState)
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
				{Kind: EffectWorkerUnresponsive, JobID: typed.JobID, State: snapshot.Job.State},
			},
		}, nil
	default:
		return transition{}, fmt.Errorf("handle event: unsupported %T", event)
	}
}

func validateTransition(next transition) error {
	if err := next.NextJob.Validate(); err != nil {
		return err
	}
	for i, effect := range next.Effects {
		if effect.Kind == "" {
			return fmt.Errorf("validate transition effect[%d]: kind is required", i)
		}
		if effect.JobID == "" {
			return fmt.Errorf("validate transition effect[%d]: job id is required", i)
		}
		if effect.Kind == EffectProbeMedia || effect.Kind == EffectBootstrapRecovery || effect.Kind == EffectDispatchRecovery || effect.Kind == EffectCheckpoint {
			if effect.Token == 0 {
				return fmt.Errorf("validate transition effect[%d]: async effect %q requires a token", i, effect.Kind)
			}
		}
	}
	return nil
}

func commitTransition(snapshot Snapshot, next transition) Snapshot {
	committed := snapshot
	committed.Job = next.NextJob
	return committed
}

func publishEffects(next transition) []Effect {
	return append([]Effect(nil), next.Effects...)
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

func buildFailedTransition(job Job, err error) transition {
	if err == nil {
		err = fmt.Errorf("job failed")
	}
	job.State = JobStateFailed
	job.ResumeState = ""
	job = clearPending(job)
	return transition{
		NextJob: job,
		Effects: []Effect{{Kind: EffectReportFailure, JobID: job.ID, State: JobStateFailed}},
	}
}

func clearPending(job Job) Job {
	job.PendingKind = ""
	job.PendingToken = 0
	return job
}

func issueEffect(job Job, kind EffectKind, state JobState) (Job, Effect) {
	if job.NextToken == 0 {
		job.NextToken = 1
	}
	token := job.NextToken
	job.NextToken++
	job.PendingKind = kind
	job.PendingToken = token
	return job, Effect{
		Kind:  kind,
		JobID: job.ID,
		State: state,
		Token: token,
	}
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
