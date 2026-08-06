package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
)

type ProgramModel struct {
	Model
	runtime platform.Runtime
	state   *programState
}

type programState struct {
	activeRecovery platform.RecoveryJob
	activeJobID    string
}

func NewProgramModel(runtime platform.Runtime) ProgramModel {
	return ProgramModel{
		Model:   NewModel(),
		runtime: runtime,
		state:   &programState{},
	}
}

func (m ProgramModel) Init() tea.Cmd {
	return m.resolve(m.Model.Init())
}

func (m ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.Model.Update(msg)
	model := next.(Model)
	nextModel := ProgramModel{Model: model, runtime: m.runtime, state: m.state}
	return nextModel, tea.Batch(nextModel.resolve(cmd), nextModel.followUp(msg))
}

func (m ProgramModel) View() tea.View {
	return m.Model.View()
}

func (m ProgramModel) resolve(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		request, ok := msg.(EffectRequestedMsg)
		if !ok {
			return msg
		}
		return m.runEffect(request)
	}
}

func (m ProgramModel) runEffect(request EffectRequestedMsg) tea.Msg {
	switch request.Kind {
	case EffectDiscoverDevices:
		drives, err := m.runtime.Optical.DiscoverOpticalDrives()
		return DevicesDiscoveredMsg{
			RequestID: request.RequestID,
			Devices:   toDeviceSummaries(drives),
			Err:       err,
		}
	case EffectIdentifyMedia:
		media, err := m.runtime.Optical.IdentifyOpticalMedia(request.DevicePath)
		if err != nil {
			return MediaIdentifiedMsg{RequestID: request.RequestID, Err: err}
		}
		return MediaIdentifiedMsg{
			RequestID:           request.RequestID,
			Identity:            ContentIdentityViewModel{Summary: media.Summary, Detail: media.Detail},
			FileSystem:          media.FileSystem,
			VolumeLabel:         media.VolumeLabel,
			LogicalSectorSize:   media.LogicalSectorSize,
			CapacitySectors:     media.CapacitySectors,
			SuggestedOutputPath: media.SuggestedOutputPath,
			Recoverable:         media.Recoverable,
			RecoverabilityNote:  media.RecoverabilityNote,
		}
	case EffectLookupHistory:
		return PriorProcessingLookupMsg{Err: fmt.Errorf("history lookup is unavailable in this build")}
	case EffectStartJob:
		jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
		job, err := m.runtime.Recovery.StartImageRecovery(platform.RecoveryInput{
			DevicePath:        m.SelectedDrive.Path,
			OutputPath:        m.Setup.OutputPath,
			LogicalSectorSize: m.MediaLogicalSectorSize,
			CapacitySectors:   m.MediaCapacitySectors,
		})
		if err != nil {
			return JobStartFailedMsg{Err: err}
		}
		m.state.activeRecovery = job
		m.state.activeJobID = jobID
		return JobStartedMsg{
			JobID:        jobID,
			OutputPath:   m.Setup.OutputPath,
			Phase:        "Reading optical sectors",
			Status:       "Reading sectors from the selected optical drive.",
			TotalSectors: m.MediaCapacitySectors,
		}
	case EffectPauseJob:
		return StatusMsg{Text: "Pause is not implemented for the current recovery backend.", Severity: SeverityWarning}
	case EffectResumeJob:
		return StatusMsg{Text: "Resume is not implemented for the current recovery backend.", Severity: SeverityWarning}
	case EffectStopJob:
		if m.state.activeRecovery == nil {
			return StatusMsg{Text: "No active recovery job is available to stop.", Severity: SeverityWarning}
		}
		m.state.activeRecovery.Cancel()
		return StatusMsg{Text: "Stopping recovery after the current read completes.", Severity: SeverityWarning}
	case EffectStopNow:
		if m.state.activeRecovery == nil {
			return StatusMsg{Text: "No active recovery job is available to stop.", Severity: SeverityWarning}
		}
		m.state.activeRecovery.Cancel()
		return StatusMsg{Text: "Immediate stop requested.", Severity: SeverityWarning}
	default:
		return FatalMsg{Err: fmt.Errorf("unsupported effect: %s", request.Kind)}
	}
}

func (m ProgramModel) followUp(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case JobStartedMsg, ProgressMsg, StatusMsg:
		if m.state != nil && m.state.activeRecovery != nil {
			return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg {
				snapshot := m.state.activeRecovery.Snapshot()
				totalSectors := m.MediaCapacitySectors
				logicalSectorSize := uint64(m.MediaLogicalSectorSize)
				if logicalSectorSize == 0 {
					logicalSectorSize = 2048
				}
				if snapshot.Done {
					m.state.activeRecovery = nil
					summary := JobSummary{
						ImagePath:         m.Setup.OutputPath,
						MapPath:           "",
						RecoveredSectors:  snapshot.CopiedBytes / logicalSectorSize,
						TotalSectors:      totalSectors,
						UnresolvedSectors: snapshot.UnreadableSectors,
						Duration:          time.Since(snapshot.StartedAt).Round(time.Second).String(),
					}
					if snapshot.Canceled {
						summary.Outcome = "Recovery stopped"
						summary.NextAction = "Review the partial image before trying again"
					} else if snapshot.ErrText != "" {
						summary.Outcome = "Recovery failed"
						summary.NextAction = "Review the error and try again"
						return JobStoppedMsg{Summary: summary, Err: errors.New(snapshot.ErrText)}
					} else if snapshot.UnreadableSectors > 0 {
						summary.Outcome = "Recovery finished with unreadable sectors"
						summary.NextAction = "Review unreadable sectors before trying again"
					} else {
						summary.Outcome = "Recovery complete"
						summary.NextAction = "Recovery image is ready"
					}
					return JobStoppedMsg{Summary: summary}
				}
				return ProgressMsg{
					Snapshot: ProgressSnapshot{
						Phase:             "Reading optical sectors",
						RecoveredSectors:  snapshot.CopiedBytes / logicalSectorSize,
						TotalSectors:      totalSectors,
						UnreadableSectors: snapshot.UnreadableSectors,
						Status:            "Reading sectors from the selected optical drive.",
						Remaining:         fmt.Sprintf("%d bytes remaining", snapshot.TotalBytes-snapshot.CopiedBytes),
						LastIssue:         append([]string(nil), snapshot.LastIssue...),
						OutputPath:        m.Setup.OutputPath,
					},
				}
			})
		}
	}
	return nil
}

func toDeviceSummaries(drives []platform.OpticalDrive) []DeviceSummary {
	items := make([]DeviceSummary, 0, len(drives))
	seen := map[string]struct{}{}
	for _, drive := range drives {
		key := strings.TrimSpace(drive.Path)
		if key == "" {
			key = strings.TrimSpace(drive.DisplayName)
		}
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		displayName := strings.TrimSpace(drive.DisplayName)
		if displayName == "" {
			displayName = drive.Path
		}
		status := strings.TrimSpace(drive.Status)
		if status == "" {
			status = "available"
		}
		items = append(items, DeviceSummary{
			Path:        drive.Path,
			DisplayName: displayName,
			Status:      status,
		})
	}
	return items
}
