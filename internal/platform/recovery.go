package platform

import "time"

type RecoveryInput struct {
	DevicePath        string
	OutputPath        string
	LogicalSectorSize uint32
	CapacitySectors   uint64
}

type RecoveryTargetStatus struct {
	OutputPath        string
	MapPath           string
	CanStartNew       bool
	CanResume         bool
	RecoveredSectors  uint64
	DeferredSectors   uint64
	UnreadableSectors uint64
	RequiredBytes     uint64
	AvailableBytes    uint64
	SpaceKnown        bool
	Detail            string
}

type RecoverySnapshot struct {
	StartedAt         time.Time
	TotalBytes        uint64
	CopiedBytes       uint64
	ScannedSectors    uint64
	DeferredSectors   uint64
	UnreadableSectors uint64
	Pass              string
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
	InspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error)
}
