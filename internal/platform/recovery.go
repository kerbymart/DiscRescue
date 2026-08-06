package platform

import "time"

type RecoveryInput struct {
	DevicePath        string
	OutputPath        string
	LogicalSectorSize uint32
	CapacitySectors   uint64
}

type RecoverySnapshot struct {
	StartedAt         time.Time
	TotalBytes        uint64
	CopiedBytes       uint64
	UnreadableSectors uint64
	MapPath           string
	Resumed           bool
	LastIssue         []string
	Done              bool
	Canceled          bool
	ErrText           string
}

type RecoveryJob interface {
	Snapshot() RecoverySnapshot
	Cancel()
}

type RecoveryService interface {
	StartImageRecovery(input RecoveryInput) (RecoveryJob, error)
}
