package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m Model) Init() tea.Cmd {
	return discoverDevicesEffect()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = typed.Width
		m.Height = typed.Height
		return m, nil
	case tea.KeyPressMsg:
		return m.handleKeyPress(typed)
	case DevicesDiscoveredMsg:
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityError}
			return m, nil
		}
		m.Devices = append([]DeviceSummary(nil), typed.Devices...)
		m.Cursor = 0
		m.Page = PageChooseDrive
		m.Notice = &NoticeModel{Text: "Choose one optical drive.", Severity: SeverityInfo}
		if len(m.Devices) == 0 {
			m.Notice = &NoticeModel{Text: "No usable optical drives found.", Severity: SeverityWarning}
		}
		return m, nil
	case MediaIdentifiedMsg:
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityError}
			return m, nil
		}
		m.Identity = typed.Identity
		m.Page = PagePriorProcessing
		return m, lookupPriorProcessingEffect()
	case PriorProcessingLookupMsg:
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityWarning}
			return m, nil
		}
		m.PriorRecords = append([]PriorProcessingRecord(nil), typed.Records...)
		if len(m.PriorRecords) == 0 {
			m.PriorRecords = []PriorProcessingRecord{{
				Title:  "History",
				Detail: "no matching contents found on this computer",
			}}
		}
		return m, nil
	case JobStartedMsg:
		m.Page = PageRecovering
		m.Recovery.Status = fmt.Sprintf("Job %s started.", typed.JobID)
		return m, nil
	case ProgressMsg:
		m.Recovery.Phase = typed.Snapshot.Phase
		m.Recovery.RecoveredSectors = typed.Snapshot.RecoveredSectors
		m.Recovery.TotalSectors = typed.Snapshot.TotalSectors
		m.Recovery.UnreadableSectors = typed.Snapshot.UnreadableSectors
		m.Recovery.Status = typed.Snapshot.Status
		return m, nil
	case StatusMsg:
		m.Notice = &NoticeModel{Text: typed.Text, Severity: typed.Severity}
		return m, nil
	case JobCheckpointedMsg:
		m.Notice = &NoticeModel{
			Text:     fmt.Sprintf("Progress saved at %s.", typed.At.Format(time.RFC822)),
			Severity: SeverityInfo,
		}
		return m, nil
	case JobStoppedMsg:
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityError}
		}
		m.Page = PageSummary
		m.Recovery.Status = typed.Summary.Outcome
		m.Details.Lines = []string{
			"Image: " + typed.Summary.ImagePath,
			"Map: " + typed.Summary.MapPath,
			"Next: " + typed.Summary.NextAction,
		}
		return m, nil
	case WorkerUnresponsiveMsg:
		m.Notice = &NoticeModel{
			Text:     fmt.Sprintf("Worker unresponsive for %s. Checkpoint requested.", typed.Since.Round(time.Second)),
			Severity: SeverityWarning,
		}
		return m, nil
	case FatalMsg:
		m.LastError = typed.Err
		if typed.Err != nil {
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityError}
		}
		m.Page = PageSummary
		return m, nil
	case EffectRequestedMsg:
		return m, nil
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())

	switch {
	case matchesKey(key, DefaultKeys().Quit):
		m.Quitting = true
		return m, tea.Quit
	case matchesKey(key, DefaultKeys().Back):
		return m.handleBack()
	case matchesKey(key, DefaultKeys().Up):
		m.moveCursor(-1)
		return m, nil
	case matchesKey(key, DefaultKeys().Down):
		m.moveCursor(1)
		return m, nil
	case matchesKey(key, DefaultKeys().Details):
		if m.Page == PageRecovering || m.Page == PageSummary {
			m.PreviousPage = m.Page
			m.Page = PageDetails
		}
		return m, nil
	case matchesKey(key, DefaultKeys().Advanced):
		if m.Page == PageChooseAction {
			m.PreviousPage = m.Page
			m.Page = PageAdvanced
		}
		return m, nil
	case matchesKey(key, DefaultKeys().Select):
		return m.handleSelect()
	default:
		return m, nil
	}
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.Page {
	case PageDetails, PageAdvanced, PageAbout, PageHistory, PageResumeJobs:
		if m.PreviousPage != 0 || m.Page == PageDetails {
			m.Page, m.PreviousPage = m.PreviousPage, 0
		}
	case PageChooseAction:
		m.Page = PagePriorProcessing
	case PageChooseOutput:
		m.Page = PageChooseAction
	case PageReview:
		m.Page = PageChooseOutput
	case PagePriorProcessing:
		m.Page = PageChooseDrive
	case PageSummary:
		m.Page = PageChooseAction
	}
	return m, nil
}

func (m Model) handleSelect() (tea.Model, tea.Cmd) {
	switch m.Page {
	case PageDiscover:
		return m, discoverDevicesEffect()
	case PageChooseDrive:
		if len(m.Devices) == 0 {
			return m, nil
		}
		selected := m.Devices[m.Cursor]
		m.Identity.Summary = "Identifying logical contents for " + selected.DisplayName
		m.Identity.Detail = selected.Path
		return m, identifyMediaEffect(selected.Path)
	case PagePriorProcessing:
		m.Page = PageChooseAction
		m.Cursor = 0
		return m, nil
	case PageChooseAction:
		switch m.Cursor {
		case 0:
			m.Page = PageChooseOutput
			return m, nil
		case 1:
			m.PreviousPage = m.Page
			m.Page = PageResumeJobs
			return m, nil
		case 2:
			m.PreviousPage = m.Page
			m.Page = PageHistory
			return m, nil
		case 3:
			m.PreviousPage = m.Page
			m.Page = PageAbout
			return m, nil
		default:
			return m, nil
		}
	case PageChooseOutput:
		m.Page = PageReview
		return m, nil
	case PageReview:
		m.Page = PageRecovering
		return m, startJobEffect()
	case PageSummary:
		m.Page = PageChooseAction
		return m, nil
	default:
		return m, nil
	}
}

func (m *Model) moveCursor(delta int) {
	limit := m.cursorLimit()
	if limit == 0 {
		m.Cursor = 0
		return
	}
	m.Cursor += delta
	if m.Cursor < 0 {
		m.Cursor = 0
	}
	if m.Cursor >= limit {
		m.Cursor = limit - 1
	}
}

func (m Model) cursorLimit() int {
	switch m.Page {
	case PageChooseDrive:
		return len(m.Devices)
	case PageChooseAction:
		return 4
	default:
		return 0
	}
}

func matchesKey(key string, options []string) bool {
	for _, option := range options {
		if key == option {
			return true
		}
	}
	return false
}

func discoverDevicesEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectDiscoverDevices}
	}
}

func identifyMediaEffect(devicePath string) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectIdentifyMedia, DevicePath: devicePath}
	}
}

func lookupPriorProcessingEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectLookupHistory}
	}
}

func startJobEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectStartJob}
	}
}
