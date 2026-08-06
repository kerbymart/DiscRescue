package app

import (
	"time"

	"discrescue/internal/platform"
)

type StartRequested struct{}

type QuitRequested struct{}

type DevicesDiscoveredMsg struct {
	RequestID int
	Devices   []DeviceSummary
	Err       error
}

type MediaIdentifiedMsg struct {
	RequestID           int
	Identity            ContentIdentityViewModel
	FileSystem          string
	VolumeLabel         string
	LogicalSectorSize   uint32
	CapacitySectors     uint64
	SuggestedOutputPath string
	Recoverable         bool
	RecoverabilityNote  string
	Err                 error
}

type PriorProcessingLookupMsg struct {
	RequestID int
	View      PriorProcessingViewModel
	Records   []PriorProcessingRecord
	Jobs      []ResumableJobViewModel
	Err       error
}

type CatalogUpdatedMsg struct {
	ContentID [32]byte
	Err       error
}

type JobStartedMsg struct {
	JobID              string
	OutputPath         string
	Phase              string
	Status             string
	TotalSectors       uint64
	RecoveredSectors   uint64
	PassCoveredSectors uint64
	PassTargetSectors  uint64
	DeferredSectors    uint64
	UnreadableSectors  uint64
}

type JobStartFailedMsg struct {
	Err error
}

type RecoveryTargetInspectedMsg struct {
	RequestID int
	Status    platform.RecoveryTargetStatus
	Err       error
}

type ResumableJobsDiscoveredMsg struct {
	RequestID int
	Jobs      []ResumableJobViewModel
	Err       error
}

type ProcessedMediaDiscoveredMsg struct {
	RequestID int
	Items     []ProcessedMediaViewModel
	Err       error
}

type ProgressMsg struct {
	Snapshot ProgressSnapshot
}

type StatusMsg struct {
	Text     string
	Severity Severity
}

type JobCheckpointedMsg struct {
	At time.Time
}

type JobPausedMsg struct {
	OutputPath         string
	MapPath            string
	RecoveredSectors   uint64
	TotalSectors       uint64
	PassCoveredSectors uint64
	PassTargetSectors  uint64
	DeferredSectors    uint64
	UnreadableSectors  uint64
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

type EffectKind string

const (
	EffectDiscoverDevices EffectKind = "discover_devices"
	EffectIdentifyMedia   EffectKind = "identify_media"
	EffectLookupHistory   EffectKind = "lookup_history"
	EffectInspectTarget   EffectKind = "inspect_target"
	EffectFindResumeJobs  EffectKind = "find_resume_jobs"
	EffectBrowseHistory   EffectKind = "browse_history"
	EffectStartJob        EffectKind = "start_job"
	EffectPauseJob        EffectKind = "pause_job"
	EffectResumeJob       EffectKind = "resume_job"
	EffectStopJob         EffectKind = "stop_job"
	EffectStopNow         EffectKind = "stop_now"
)

type EffectRequestedMsg struct {
	Kind       EffectKind
	DevicePath string
	OutputPath string
	BasePath   string
	RequestID  int
}
