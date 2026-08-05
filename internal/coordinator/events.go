package coordinator

type Event interface {
	eventName() string
}

type StartJob struct {
	JobID string
}

func (StartJob) eventName() string { return "start-job" }

type ProbeCompleted struct {
	JobID string
	Err   error
}

func (ProbeCompleted) eventName() string { return "probe-completed" }

type ResumeJob struct {
	JobID string
}

func (ResumeJob) eventName() string { return "resume-job" }

type PauseJob struct {
	JobID string
}

func (PauseJob) eventName() string { return "pause-job" }

type CancelJob struct {
	JobID string
}

func (CancelJob) eventName() string { return "cancel-job" }

type WorkerResultReceived struct {
	JobID     string
	NextState JobState
	Err       error
}

func (WorkerResultReceived) eventName() string { return "worker-result-received" }

type WriteResultReceived struct {
	JobID string
	Err   error
}

func (WriteResultReceived) eventName() string { return "write-result-received" }

type JobCheckpointed struct {
	JobID string
}

func (JobCheckpointed) eventName() string { return "job-checkpointed" }

type JobFailed struct {
	JobID string
	Err   error
}

func (JobFailed) eventName() string { return "job-failed" }

type WorkerUnresponsiveDetected struct {
	JobID  string
	Reason string
}

func (WorkerUnresponsiveDetected) eventName() string { return "worker-unresponsive-detected" }
