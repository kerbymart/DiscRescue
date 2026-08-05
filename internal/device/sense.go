package device

type SenseClass string

const (
	SenseRetryable SenseClass = "retryable"
	SenseFatal     SenseClass = "fatal"
)
