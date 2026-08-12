package coordinator

type EffectKind string

const (
	EffectProbeMedia         EffectKind = "probe_media"
	EffectBootstrapRecovery  EffectKind = "bootstrap_recovery"
	EffectDispatchRecovery   EffectKind = "dispatch_recovery"
	EffectCheckpoint         EffectKind = "checkpoint"
	EffectPublishPause       EffectKind = "publish_pause"
	EffectStopAfterSave      EffectKind = "stop_after_save"
	EffectReportFailure      EffectKind = "report_failure"
	EffectWorkerUnresponsive EffectKind = "worker_unresponsive"
)

type Effect struct {
	Kind  EffectKind
	JobID string
	State JobState
	Token uint64
}
