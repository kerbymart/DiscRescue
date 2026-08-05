package app

type StartRequested struct{}

type QuitRequested struct{}

type WorkerStatusReceived struct {
	Status string
}
