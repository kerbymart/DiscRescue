package platform

import (
	"time"

	"discrescue/internal/recovery"
)

type RecoveryMethod = recovery.RecoveryMethod

const (
	RecoveryMethodFast     = recovery.RecoveryMethodFast
	RecoveryMethodBalanced = recovery.RecoveryMethodBalanced
	RecoveryMethodGentle   = recovery.RecoveryMethodGentle
	StopIntentPause        = recovery.StopIntentPause
	StopIntentStop         = recovery.StopIntentStop
)

type StopIntent = recovery.StopIntent

type RecoveryInput struct {
	DevicePath        string
	OutputPath        string
	LogicalSectorSize uint32
	CapacitySectors   uint64
	Method            RecoveryMethod
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
	State                    recovery.JobState
	StartedAt                time.Time
	EndedAt                  time.Time
	Method                   RecoveryMethod
	TotalBytes               uint64
	CopiedBytes              uint64
	CumulativeRecoveredBytes uint64
	SessionRecoveredBytes    uint64
	ScannedSectors           uint64
	DeferredSectors          uint64
	UnreadableSectors        uint64
	Pass                     string
	MapPath                  string
	Resumed                  bool
	LastIssue                []string
	Done                     bool
	Canceled                 bool
	ErrText                  string
	StopIntent               recovery.StopIntent
	CanForceStop             bool
}

type RecoveryJob interface {
	Snapshot() RecoverySnapshot
	Cancel()
}

// StoppableRecoveryJob exposes bounded stop intent and escalation when the
// platform implementation supports it. RecoveryJob remains source-compatible
// with adapters that have not migrated to the lifecycle controller yet.
type StoppableRecoveryJob interface {
	RecoveryJob
	RequestStop(recovery.StopIntent) error
	ForceStop() error
}

type RecoveryService interface {
	StartImageRecovery(input RecoveryInput) (RecoveryJob, error)
	InspectRecoveryTarget(input RecoveryInput) (RecoveryTargetStatus, error)
}
