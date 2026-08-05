package coordinator

import "fmt"

type ShutdownReason string

const (
	ShutdownPause      ShutdownReason = "pause"
	ShutdownCancel     ShutdownReason = "cancel"
	ShutdownFatalError ShutdownReason = "fatal_error"
)

type ShutdownStep string

const (
	ShutdownStopScheduling    ShutdownStep = "stop_scheduling"
	ShutdownCancelWorker      ShutdownStep = "cancel_or_finish_worker"
	ShutdownDrainImage        ShutdownStep = "drain_image_batches"
	ShutdownCommitMap         ShutdownStep = "commit_map_transitions"
	ShutdownFinalCheckpoint   ShutdownStep = "write_final_checkpoint"
	ShutdownCatalogBestEffort ShutdownStep = "record_catalog_state_best_effort"
	ShutdownCloseImageMap     ShutdownStep = "close_image_and_map"
	ShutdownCloseCatalog      ShutdownStep = "close_catalog"
	ShutdownStopWorker        ShutdownStep = "stop_worker"
	ShutdownStopLogWriter     ShutdownStep = "stop_log_writer"
	ShutdownNotifyTUI         ShutdownStep = "notify_tui"
	ShutdownRestoreTerminal   ShutdownStep = "restore_terminal"
)

type ShutdownPlan struct {
	Reason ShutdownReason
	Steps  []ShutdownStep
}

func BuildShutdownPlan(reason ShutdownReason) (ShutdownPlan, error) {
	if reason == "" {
		return ShutdownPlan{}, fmt.Errorf("build shutdown plan: reason is required")
	}

	steps := []ShutdownStep{
		ShutdownStopScheduling,
		ShutdownCancelWorker,
		ShutdownDrainImage,
		ShutdownCommitMap,
		ShutdownFinalCheckpoint,
		ShutdownCatalogBestEffort,
		ShutdownCloseImageMap,
		ShutdownCloseCatalog,
		ShutdownStopWorker,
		ShutdownStopLogWriter,
		ShutdownNotifyTUI,
		ShutdownRestoreTerminal,
	}

	switch reason {
	case ShutdownPause:
		steps = removeShutdownStep(steps, ShutdownCloseCatalog)
		steps = removeShutdownStep(steps, ShutdownStopLogWriter)
	case ShutdownCancel, ShutdownFatalError:
	default:
		return ShutdownPlan{}, fmt.Errorf("build shutdown plan: unsupported reason %q", reason)
	}

	return ShutdownPlan{
		Reason: reason,
		Steps:  steps,
	}, nil
}

func removeShutdownStep(steps []ShutdownStep, target ShutdownStep) []ShutdownStep {
	filtered := make([]ShutdownStep, 0, len(steps))
	for _, step := range steps {
		if step == target {
			continue
		}
		filtered = append(filtered, step)
	}
	return filtered
}
