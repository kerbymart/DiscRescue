package coordinator

type Event interface {
	eventName() string
}

type StartJob struct {
	JobID string
}

func (StartJob) eventName() string { return "start-job" }

type PauseJob struct {
	JobID string
}

func (PauseJob) eventName() string { return "pause-job" }

type CancelJob struct {
	JobID string
}

func (CancelJob) eventName() string { return "cancel-job" }
