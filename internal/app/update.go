package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/platform"
)

func (m Model) Init() tea.Cmd {
	return discoverDevicesEffect(m.ActiveDiscoveryRequest)
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
		if typed.RequestID != m.ActiveDiscoveryRequest {
			return m, nil
		}
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Page = PageDiscoveryError
			if errors.Is(typed.Err, platform.ErrUnsupportedEnvironment) {
				m.Notice = &NoticeModel{Text: "Optical drive discovery is not supported in this environment.", Severity: SeverityWarning}
				return m, nil
			}
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityError}
			return m, nil
		}
		m.Devices = append([]DeviceSummary(nil), typed.Devices...)
		m.Cursor = 0
		if len(m.Devices) == 0 {
			m.Page = PageNoDrives
			m.Notice = &NoticeModel{Text: "No usable optical drives found.", Severity: SeverityWarning}
			return m, nil
		}
		m.Page = PageChooseDrive
		m.Notice = &NoticeModel{Text: "Choose one optical drive.", Severity: SeverityInfo}
		return m, nil
	case MediaIdentifiedMsg:
		if typed.RequestID != m.ActiveMediaRequest {
			return m, nil
		}
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Page = PageInspectingMedia
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
		m.PriorView = typed.View
		m.PriorRecords = append([]PriorProcessingRecord(nil), typed.Records...)
		if m.PriorView.Kind == "" {
			m.PriorView = defaultPriorProcessingView()
		}
		if m.PriorView.Kind == PriorProcessingNone && len(m.PriorRecords) == 0 {
			m.PriorRecords = []PriorProcessingRecord{{Title: "History", Detail: "no matching contents found on this computer"}}
		}
		return m, nil
	case JobStartedMsg:
		m.Page = PageRecovering
		m.Recovery.Status = fmt.Sprintf("Job %s started.", typed.JobID)
		m.Recovery.OutputPath = m.Setup.OutputPath
		return m, nil
	case ProgressMsg:
		m.Recovery.Phase = typed.Snapshot.Phase
		m.Recovery.RecoveredSectors = typed.Snapshot.RecoveredSectors
		m.Recovery.TotalSectors = typed.Snapshot.TotalSectors
		m.Recovery.UnreadableSectors = typed.Snapshot.UnreadableSectors
		m.Recovery.Status = typed.Snapshot.Status
		m.Recovery.Remaining = typed.Snapshot.Remaining
		m.Recovery.ETA = typed.Snapshot.ETA
		m.Recovery.LastIssue = append([]string(nil), typed.Snapshot.LastIssue...)
		m.Recovery.PausePending = typed.Snapshot.PausePending
		if typed.Snapshot.OutputPath != "" {
			m.Recovery.OutputPath = typed.Snapshot.OutputPath
		}
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
		m.Summary = typed.Summary
		m.Recovery.Status = typed.Summary.Outcome
		m.Recovery.RecoveredSectors = typed.Summary.RecoveredSectors
		m.Recovery.TotalSectors = typed.Summary.TotalSectors
		m.Recovery.UnreadableSectors = typed.Summary.UnresolvedSectors
		m.Details.Lines = []string{
			"Image: " + typed.Summary.ImagePath,
			"Map: " + typed.Summary.MapPath,
			"Next: " + typed.Summary.NextAction,
		}
		m.Cursor = 0
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
	case matchesKey(key, DefaultKeys().Force):
		switch m.Page {
		case PageRecovering, PagePaused:
			m.PreviousPage = m.Page
			m.Page = PageStopConfirm
			m.Cursor = 0
			return m, nil
		case PageStopConfirm:
			return m, stopImmediatelyEffect()
		default:
			m.Quitting = true
			return m, tea.Quit
		}
	case matchesKey(key, DefaultKeys().Quit):
		switch m.Page {
		case PageRecovering, PagePaused:
			m.PreviousPage = m.Page
			m.Page = PageStopConfirm
			m.Cursor = 0
			return m, nil
		default:
			m.Quitting = true
			return m, tea.Quit
		}
	case matchesKey(key, DefaultKeys().Back):
		return m.handleBack()
	case matchesKey(key, DefaultKeys().Up):
		m.moveCursor(-1)
		return m, nil
	case matchesKey(key, DefaultKeys().Down):
		m.moveCursor(1)
		return m, nil
	case matchesKey(key, DefaultKeys().Pause):
		switch m.Page {
		case PageRecovering:
			m.Page = PagePaused
			m.Cursor = 0
			return m, pauseJobEffect()
		case PagePaused:
			m.Page = PageRecovering
			m.Cursor = 0
			return m, resumeJobEffect()
		default:
			return m, nil
		}
	case matchesKey(key, DefaultKeys().Details):
		if m.Page == PageRecovering || m.Page == PagePaused || m.Page == PageSummary {
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
	case PageStopConfirm:
		if m.PreviousPage == PagePaused {
			m.Page = PagePaused
		} else {
			m.Page = PageRecovering
		}
		m.Cursor = 0
	case PageChooseAction:
		m.Page = PagePriorProcessing
	case PageChooseOutput:
		m.Page = PageChooseAction
	case PageReview:
		m.Page = PageChooseAction
		m.Cursor = 0
	case PagePriorProcessing:
		m.Page = PageChooseDrive
	case PageInspectingMedia:
		m.ActiveMediaRequest = 0
		m.Page = PageChooseDrive
	case PageSummary:
		m.Page = PageChooseAction
		m.Cursor = 0
	}
	return m, nil
}

func (m Model) handleSelect() (tea.Model, tea.Cmd) {
	switch m.Page {
	case PageDiscover:
		return m.beginDiscovery()
	case PageNoDrives, PageDiscoveryError:
		return m.beginDiscovery()
	case PageChooseDrive:
		if len(m.Devices) == 0 {
			return m, nil
		}
		selected := m.Devices[m.Cursor]
		m.SelectedDrive = selected
		m.Page = PageInspectingMedia
		m.Identity.Summary = "Identifying logical contents for " + selected.DisplayName
		m.Identity.Detail = selected.Path
		requestID := m.nextRequestID()
		m.ActiveMediaRequest = requestID
		return m, identifyMediaEffect(selected.Path, requestID)
	case PageInspectingMedia:
		if m.SelectedDrive.Path == "" {
			return m, nil
		}
		requestID := m.nextRequestID()
		m.ActiveMediaRequest = requestID
		m.Notice = &NoticeModel{Text: "Inspecting the selected media.", Severity: SeverityInfo}
		return m, identifyMediaEffect(m.SelectedDrive.Path, requestID)
	case PagePriorProcessing:
		switch m.PriorView.Kind {
		case PriorProcessingStrongCompleted, PriorProcessingStrongResumable, PriorProcessingProbable:
			switch m.Cursor {
			case 0:
				m.Page = PageChooseAction
				m.Cursor = 0
				return m, nil
			case len(m.PriorView.Options) - 1:
				m.Page = PageChooseDrive
				m.Cursor = 0
				return m, nil
			default:
				m.Page = PageChooseAction
				m.Cursor = 0
				return m, nil
			}
		default:
			m.Page = PageChooseAction
			m.Cursor = 0
			return m, nil
		}
	case PageChooseAction:
		switch m.Cursor {
		case 0:
			m.Setup.ActionLabel = "Start a new recovery"
			m.Page = PageReview
			return m, nil
		case 1:
			m.PreviousPage = m.Page
			m.Page = PageResumeJobs
			return m, nil
		case 2:
			m.Setup.ActionLabel = "Verify an existing image"
			m.Page = PageChooseOutput
			return m, nil
		case 3:
			m.Setup.ActionLabel = "Merge recovery captures"
			m.Page = PageChooseOutput
			return m, nil
		case 4:
			m.PreviousPage = m.Page
			m.Page = PageHistory
			return m, nil
		default:
			return m, nil
		}
	case PageChooseOutput:
		m.Page = PageReview
		return m, nil
	case PageReview:
		switch m.Cursor {
		case 0:
			m.Page = PageRecovering
			return m, startJobEffect()
		case 1:
			m.Page = PageChooseOutput
			m.Cursor = 0
			return m, nil
		case 2:
			m.Setup.MethodLabel = nextMethodLabel(m.Setup.MethodLabel)
			return m, nil
		case 3:
			m.Setup.CopyLabel = nextCopyLabel(m.Setup.CopyLabel)
			return m, nil
		case 4:
			m.PreviousPage = m.Page
			m.Page = PageAdvanced
			return m, nil
		default:
			return m, nil
		}
	case PagePaused:
		switch m.Cursor {
		case 0:
			m.Page = PageRecovering
			m.Cursor = 0
			return m, resumeJobEffect()
		case 1:
			m.PreviousPage = PagePaused
			m.Page = PageStopConfirm
			m.Cursor = 0
			return m, nil
		default:
			return m, nil
		}
	case PageStopConfirm:
		switch m.Cursor {
		case 0:
			return m, stopAfterCheckpointEffect()
		case 1:
			if m.PreviousPage == PagePaused {
				m.Page = PagePaused
			} else {
				m.Page = PageRecovering
			}
			m.Cursor = 0
			return m, nil
		case 2:
			return m, stopImmediatelyEffect()
		default:
			return m, nil
		}
	case PageSummary:
		switch m.Cursor {
		case 0:
			m.Page = PageChooseAction
			m.Cursor = 0
			return m, nil
		case 1:
			m.Setup.ActionLabel = summarySecondaryActionLabel(m)
			m.Page = PageChooseOutput
			m.Cursor = 0
			return m, nil
		case 2:
			m.PreviousPage = m.Page
			m.Page = PageDetails
			return m, nil
		default:
			return m, nil
		}
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
		return 5
	case PagePriorProcessing:
		return len(m.PriorView.Options)
	case PageReview:
		return 5
	case PagePaused:
		return 2
	case PageStopConfirm:
		return 3
	case PageSummary:
		return 3
	default:
		return 0
	}
}

func (m *Model) nextRequestID() int {
	requestID := m.NextRequestID
	if requestID == 0 {
		requestID = 1
	}
	m.NextRequestID = requestID + 1
	return requestID
}

func (m Model) beginDiscovery() (tea.Model, tea.Cmd) {
	m.Page = PageDiscover
	m.LastError = nil
	m.Notice = &NoticeModel{Text: "Finding usable drives and resumable jobs.", Severity: SeverityInfo}
	requestID := m.nextRequestID()
	m.ActiveDiscoveryRequest = requestID
	m.ActiveMediaRequest = 0
	return m, discoverDevicesEffect(requestID)
}

func matchesKey(key string, options []string) bool {
	for _, option := range options {
		if key == option {
			return true
		}
	}
	return false
}

func discoverDevicesEffect(requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectDiscoverDevices, RequestID: requestID}
	}
}

func identifyMediaEffect(devicePath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectIdentifyMedia, DevicePath: devicePath, RequestID: requestID}
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

func pauseJobEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectPauseJob}
	}
}

func resumeJobEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectResumeJob}
	}
}

func stopAfterCheckpointEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectStopJob}
	}
}

func stopImmediatelyEffect() tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectStopNow}
	}
}

func defaultPriorProcessingView() PriorProcessingViewModel {
	return PriorProcessingViewModel{
		Kind:        PriorProcessingNone,
		HistoryLine: "History: no matching contents found on this computer",
	}
}

func nextMethodLabel(current string) string {
	switch current {
	case "Balanced recovery":
		return "Fast recovery"
	case "Fast recovery":
		return "Gentle recovery"
	default:
		return "Balanced recovery"
	}
}

func nextCopyLabel(current string) string {
	if current == "Not set (optional)" {
		return "Shelf B · Disc 14"
	}
	return "Not set (optional)"
}

func summarySecondaryActionLabel(m Model) string {
	if m.Recovery.UnreadableSectors > 0 {
		return "Retry unreadable sectors"
	}
	return "Verify an existing image"
}
