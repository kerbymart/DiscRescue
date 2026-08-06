package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
)

type ProgramModel struct {
	Model
	runtime platform.Runtime
}

func NewProgramModel(runtime platform.Runtime) ProgramModel {
	return ProgramModel{
		Model:   NewModel(),
		runtime: runtime,
	}
}

func (m ProgramModel) Init() tea.Cmd {
	return m.resolve(m.Model.Init())
}

func (m ProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.Model.Update(msg)
	model := next.(Model)
	return ProgramModel{Model: model, runtime: m.runtime}, m.resolve(cmd)
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
			RequestID: request.RequestID,
			Identity: ContentIdentityViewModel{
				Summary: media.Summary,
				Detail:  media.Detail,
			},
		}
	case EffectLookupHistory:
		return PriorProcessingLookupMsg{
			View: defaultPriorProcessingView(),
			Records: []PriorProcessingRecord{
				{Title: "History", Detail: "no matching contents found on this computer"},
			},
		}
	case EffectStartJob:
		return JobStartedMsg{JobID: "startup-demo"}
	case EffectPauseJob:
		return StatusMsg{Text: "Pause requested. Waiting for the current drive request to finish if needed.", Severity: SeverityInfo}
	case EffectResumeJob:
		return StatusMsg{Text: "Recovery resumed.", Severity: SeverityInfo}
	case EffectStopJob:
		return JobStoppedMsg{Summary: JobSummary{
			Outcome:           "Recovery stopped",
			ImagePath:         m.Recovery.OutputPath,
			MapPath:           replaceExtension(m.Recovery.OutputPath, ".drmap"),
			NextAction:        "Resume later",
			RecoveredSectors:  m.Recovery.RecoveredSectors,
			TotalSectors:      m.Recovery.TotalSectors,
			UnresolvedSectors: m.Recovery.UnreadableSectors,
			CatalogStatus:     "Not updated",
		}}
	case EffectStopNow:
		return JobStoppedMsg{Summary: JobSummary{
			Outcome:           "Recovery stopped immediately",
			ImagePath:         m.Recovery.OutputPath,
			MapPath:           replaceExtension(m.Recovery.OutputPath, ".drmap"),
			NextAction:        "Review the saved state before resuming",
			RecoveredSectors:  m.Recovery.RecoveredSectors,
			TotalSectors:      m.Recovery.TotalSectors,
			UnresolvedSectors: m.Recovery.UnreadableSectors,
			CatalogStatus:     "Not updated",
		}}
	default:
		return FatalMsg{Err: fmt.Errorf("unsupported effect: %s", request.Kind)}
	}
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
