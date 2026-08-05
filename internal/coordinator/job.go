package coordinator

type JobState string

const (
	JobStateIdle    JobState = "idle"
	JobStateRunning JobState = "running"
	JobStatePaused  JobState = "paused"
	JobStateFailed  JobState = "failed"
)

type Job struct {
	ID    string
	State JobState
}
