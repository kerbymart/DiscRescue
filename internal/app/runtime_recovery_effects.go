package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
)

func (m ProgramModel) runRecoveryEffect(request EffectRequestedMsg) tea.Msg {
	switch request.Kind {
	case EffectInspectTarget:
		status, err := m.services.Recovery.InspectRecoveryTarget(platform.RecoveryInput{DevicePath: m.SelectedDrive.Path, OutputPath: request.OutputPath, LogicalSectorSize: m.MediaLogicalSectorSize, CapacitySectors: m.MediaCapacitySectors})
		return RecoveryTargetInspectedMsg{RequestID: request.RequestID, Status: status, Err: err}
	case EffectStartJob, EffectResumeJob:
		if request.Kind == EffectResumeJob && m.state.activeRecovery != nil {
			return StatusMsg{Text: "Recovery is already running.", Severity: SeverityWarning}
		}
		return m.startRecoveryJob()
	case EffectPauseJob:
		if m.state.activeRecovery == nil {
			return StatusMsg{Text: "No active recovery job is available to pause.", Severity: SeverityWarning}
		}
		m.state.pendingPause = true
		if job, ok := m.state.activeRecovery.(platform.StoppableRecoveryJob); ok {
			if err := job.RequestStop(platform.StopIntentPause); err != nil {
				return StatusMsg{Err: err, Context: contextRecovery}
			}
			return StatusMsg{Text: "Pause requested; saving durable recovery state.", Severity: SeverityInfo}
		}
		m.state.activeRecovery.Cancel()
		return StatusMsg{Text: "Pause requested after the current read completes.", Severity: SeverityInfo}
	case EffectStopJob:
		if m.state.activeRecovery == nil {
			return StatusMsg{Text: "No active recovery job is available to stop.", Severity: SeverityWarning}
		}
		m.state.pendingPause = false
		if job, ok := m.state.activeRecovery.(platform.StoppableRecoveryJob); ok {
			if err := job.RequestStop(platform.StopIntentStop); err != nil {
				return StatusMsg{Err: err, Context: contextRecovery}
			}
			return StatusMsg{Text: "Stop requested; saving durable recovery state.", Severity: SeverityWarning}
		}
		m.state.activeRecovery.Cancel()
		return StatusMsg{Text: "Stop requested after the current read completes.", Severity: SeverityWarning}
	case EffectStopNow:
		if m.state.activeRecovery == nil {
			return StatusMsg{Text: "No active recovery job is available to stop.", Severity: SeverityWarning}
		}
		job, ok := m.state.activeRecovery.(platform.StoppableRecoveryJob)
		if !ok || !job.Snapshot().CanForceStop {
			return StatusMsg{Text: "Force stop is unavailable until the recovery worker needs escalation.", Severity: SeverityWarning}
		}
		m.state.pendingPause = false
		if err := job.ForceStop(); err != nil {
			return StatusMsg{Err: err, Context: contextRecovery}
		}
		return StatusMsg{Text: "Force-stopping the active device request; durable state is preserved.", Severity: SeverityWarning}
	default:
		return unsupportedEffect(request)
	}
}

func unsupportedEffect(request EffectRequestedMsg) tea.Msg {
	return FatalMsg{Err: fmt.Errorf("unsupported effect: %s", request.Kind)}
}
