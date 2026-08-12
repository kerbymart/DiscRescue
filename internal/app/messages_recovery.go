package app

import (
	"time"

	"discrescue/internal/device"
)

type JobStartedMsg struct {
	JobID             string
	OutputPath        string
	Phase             string
	Status            string
	TotalSectors      uint64
	ScannedSectors    uint64
	RecoveredSectors  uint64
	DeferredSectors   uint64
	UnreadableSectors uint64
}

type JobStartFailedMsg struct {
	Err error
}

type ProgressMsg struct {
	Snapshot ProgressSnapshot
}

type StatusMsg struct {
	Text     string
	Severity Severity
	Err      error
	Context  messageContext
}

type JobCheckpointedMsg struct {
	At time.Time
}

type JobPausedMsg struct {
	OutputPath        string
	MapPath           string
	ScannedSectors    uint64
	RecoveredSectors  uint64
	DeferredSectors   uint64
	TotalSectors      uint64
	UnreadableSectors uint64
}

type JobStoppedMsg struct {
	Summary JobSummary
	Err     error
}

type WorkerUnresponsiveMsg struct {
	Since time.Duration
}

type FatalMsg struct {
	Err error
}

type EjectCompletedMsg struct {
	Request device.EjectRequest
	Result  device.EjectResult
	Err     error
}
