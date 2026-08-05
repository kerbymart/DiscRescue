package coordinator

import "fmt"

type JobState string

const (
	JobStateIdle         JobState = "idle"
	JobStateIdentifying  JobState = "identifying"
	JobStatePreparing    JobState = "preparing"
	JobStateFastPass     JobState = "fast_pass"
	JobStateTrimPass     JobState = "trim_pass"
	JobStateAdaptivePass JobState = "adaptive_pass"
	JobStateScrapePass   JobState = "scrape_pass"
	JobStateVerifying    JobState = "verifying"
	JobStatePausing      JobState = "pausing"
	JobStatePaused       JobState = "paused"
	JobStateStopping     JobState = "stopping"
	JobStateCompleted    JobState = "completed"
	JobStateIncomplete   JobState = "incomplete"
	JobStateFailed       JobState = "failed"
)

type Job struct {
	ID          string
	State       JobState
	ResumeState JobState
}

func (j Job) Validate() error {
	if j.State == "" {
		return fmt.Errorf("validate job: state is required")
	}
	if j.State == JobStateIdle {
		if j.ID != "" || j.ResumeState != "" {
			return fmt.Errorf("validate job: idle job must not retain id or resume state")
		}
		return nil
	}
	if j.ID == "" {
		return fmt.Errorf("validate job: id is required outside idle state")
	}
	if !validJobState(j.State) {
		return fmt.Errorf("validate job: unsupported state %q", j.State)
	}
	switch j.State {
	case JobStatePausing, JobStatePaused, JobStateStopping:
		if !runningPhase(j.ResumeState) {
			return fmt.Errorf("validate job: state %q requires a resumable phase, got %q", j.State, j.ResumeState)
		}
	default:
		if j.ResumeState != "" {
			return fmt.Errorf("validate job: non-paused state %q must not retain resume state", j.State)
		}
	}
	return nil
}

func validJobState(state JobState) bool {
	switch state {
	case JobStateIdle,
		JobStateIdentifying,
		JobStatePreparing,
		JobStateFastPass,
		JobStateTrimPass,
		JobStateAdaptivePass,
		JobStateScrapePass,
		JobStateVerifying,
		JobStatePausing,
		JobStatePaused,
		JobStateStopping,
		JobStateCompleted,
		JobStateIncomplete,
		JobStateFailed:
		return true
	default:
		return false
	}
}

func runningPhase(state JobState) bool {
	switch state {
	case JobStateFastPass,
		JobStateTrimPass,
		JobStateAdaptivePass,
		JobStateScrapePass,
		JobStateVerifying:
		return true
	default:
		return false
	}
}

func activeJobState(state JobState) bool {
	switch state {
	case JobStateIdentifying,
		JobStatePreparing,
		JobStateFastPass,
		JobStateTrimPass,
		JobStateAdaptivePass,
		JobStateScrapePass,
		JobStateVerifying,
		JobStatePausing,
		JobStatePaused,
		JobStateStopping:
		return true
	default:
		return false
	}
}
