package coordinator

type ShutdownStep string

const (
	ShutdownCheckpoint ShutdownStep = "checkpoint"
	ShutdownWorker     ShutdownStep = "worker"
	ShutdownTerminal   ShutdownStep = "terminal"
)
