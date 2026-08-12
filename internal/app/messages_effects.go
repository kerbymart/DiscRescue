package app

import "discrescue/internal/device"

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
	EffectEject           EffectKind = "eject"
)

type EffectRequestedMsg struct {
	Kind       EffectKind
	DevicePath string
	OutputPath string
	BasePath   string
	RequestID  int
	Eject      device.EjectRequest
}
