package coordinator

import "fmt"

type transition struct {
	NextJob Job
	Effects []Effect
	Noop    bool
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
	return job, Effect{Kind: kind, JobID: job.ID, State: state, Token: token}
}

func transitionAllowed(current, next JobState) bool {
	switch current {
	case JobStateFastPass:
		return next == JobStateFastPass || next == JobStateTrimPass || next == JobStateAdaptivePass || next == JobStateScrapePass || next == JobStateVerifying
	case JobStateTrimPass:
		return next == JobStateTrimPass || next == JobStateAdaptivePass || next == JobStateScrapePass || next == JobStateVerifying
	case JobStateAdaptivePass:
		return next == JobStateAdaptivePass || next == JobStateScrapePass || next == JobStateVerifying
	case JobStateScrapePass:
		return next == JobStateScrapePass || next == JobStateVerifying
	case JobStateVerifying:
		return next == JobStateCompleted || next == JobStateIncomplete
	default:
		return false
	}
}
