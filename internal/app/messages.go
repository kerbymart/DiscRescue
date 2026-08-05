package app

import "time"

type StartRequested struct{}

type QuitRequested struct{}

type DevicesDiscoveredMsg struct {
	Devices []DeviceSummary
	Err     error
}

type MediaIdentifiedMsg struct {
	Identity ContentIdentityViewModel
	Err      error
}

type PriorProcessingLookupMsg struct {
	Records []PriorProcessingRecord
	Err     error
}

type CatalogUpdatedMsg struct {
	ContentID [32]byte
	Err       error
}

type JobStartedMsg struct {
	JobID string
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
	EffectStartJob        EffectKind = "start_job"
)

type EffectRequestedMsg struct {
	Kind       EffectKind
	DevicePath string
}
