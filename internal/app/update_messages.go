package app

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func (m Model) updateMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		return m.updateWindowMessage(typed)
	case tea.KeyPressMsg:
		return m.updateKeyMessage(typed)
	case spinner.TickMsg:
		return m.updateSpinnerMessage(typed)
	case DevicesDiscoveredMsg:
		return m.updateDevicesDiscovered(typed)
	case MediaIdentifiedMsg:
		return m.updateMediaIdentified(typed)
	case PriorProcessingLookupMsg:
		return m.updatePriorProcessingLookup(typed)
	case ProcessedMediaDiscoveredMsg:
		return m.updateProcessedMediaDiscovered(typed)
	case RecoveryTargetInspectedMsg:
		return m.updateRecoveryTargetInspected(typed)
	case ResumableJobsDiscoveredMsg:
		return m.updateResumableJobsDiscovered(typed)
	case JobStartedMsg:
		return m.updateJobStarted(typed)
	case JobPausedMsg:
		return m.updateJobPaused(typed)
	case JobStartFailedMsg:
		return m.updateJobStartFailed(typed)
	case ProgressMsg:
		return m.updateProgress(typed)
	case StatusMsg:
		return m.updateStatus(typed)
	case JobCheckpointedMsg:
		return m.updateJobCheckpointed(typed)
	case JobStoppedMsg:
		return m.updateJobStopped(typed)
	case WorkerUnresponsiveMsg:
		return m.updateWorkerUnresponsive(typed)
	case FatalMsg:
		return m.updateFatal(typed)
	case EjectCompletedMsg:
		return m.updateEjectCompleted(typed)
	case EffectRequestedMsg:
		return m, nil
	default:
		return m, nil
	}
}
