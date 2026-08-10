package platform

import (
	"time"

	"discrescue/internal/catalog"
	"discrescue/internal/device"
	"discrescue/internal/mapfile"
	"discrescue/internal/recovery"
	"discrescue/internal/recoverymap"
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
	// RetryUnresolved starts one additional bounded recovery cycle for the
	// unresolved extents already recorded in the map. It never revisits
	// recovered extents or replaces the durable attempt history.
	RetryUnresolved bool
	ReadSpeed       device.ReadSpeedRequest
	Identity        *catalog.ContentIdentity
	CaptureID       catalog.RecordID
	JobID           catalog.RecordID
	CatalogRecordID catalog.RecordID
}

func recoveryMapHeader(input RecoveryInput) (mapfile.Header, error) {
	header := mapfile.Header{
		LogicalSectorSize:   input.LogicalSectorSize,
		ExpectedSectorCount: input.CapacitySectors,
		OutputFormat:        1,
		CreationUnixNano:    time.Now().UnixNano(),
	}
	if input.Identity == nil {
		return header, nil
	}
	return recoverymap.IdentityBinding(header, *input.Identity, input.CaptureID, input.JobID, input.CatalogRecordID)
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
	Telemetry                recovery.SessionTelemetry
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
