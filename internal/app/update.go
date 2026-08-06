package app

import (
	"errors"
	"fmt"
	"path/filepath"
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
		m.MediaFileSystem = typed.FileSystem
		m.MediaVolumeLabel = typed.VolumeLabel
		m.MediaLogicalSectorSize = typed.LogicalSectorSize
		m.MediaCapacitySectors = typed.CapacitySectors
		m.MediaRecoverable = typed.Recoverable
		m.MediaRecoverabilityNote = typed.RecoverabilityNote
		if typed.SuggestedOutputPath != "" {
			applyOutputPath(&m.Setup, typed.SuggestedOutputPath)
			m.Setup.DefaultPath = typed.SuggestedOutputPath
		}
		m.Page = PageChooseAction
		m.Cursor = 0
		m.PriorView = defaultPriorProcessingView()
		m.PriorRecords = nil
		requestID := m.nextRequestID()
		m.ActiveLookupRequest = requestID
		if typed.Recoverable {
			m.Notice = &NoticeModel{Text: "Media inspection completed. Start a new recovery to create an image.", Severity: SeverityInfo}
		} else {
			m.Notice = &NoticeModel{Text: typed.RecoverabilityNote, Severity: SeverityWarning}
		}
		return m, lookupHistoryEffect(firstNonEmpty(strings.TrimSpace(m.Setup.OutputDirectory), "."), requestID)
	case PriorProcessingLookupMsg:
		if typed.RequestID != m.ActiveLookupRequest {
			return m, nil
		}
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityWarning}
			m.Page = PageChooseAction
			m.Cursor = 0
			return m, nil
		}
		if typed.View.HistoryLine != "" {
			m.PriorView = typed.View
		}
		m.PriorRecords = append([]PriorProcessingRecord(nil), typed.Records...)
		return m, nil
	case ProcessedMediaDiscoveredMsg:
		if typed.RequestID != m.ActiveHistoryRequest {
			return m, nil
		}
		if typed.Err != nil {
			m.LastError = typed.Err
			m.HistoryItems = nil
			m.Page = PageChooseAction
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityWarning}
			return m, nil
		}
		m.HistoryItems = append([]ProcessedMediaViewModel(nil), typed.Items...)
		m.Page = PageHistory
		m.Cursor = 0
		if len(m.HistoryItems) == 0 {
			m.Notice = &NoticeModel{Text: "No processed media were found in the current output folder.", Severity: SeverityInfo}
		} else {
			m.Notice = &NoticeModel{Text: "Browse saved images and recovery maps from the current output folder.", Severity: SeverityInfo}
		}
		return m, nil
	case RecoveryTargetInspectedMsg:
		if typed.RequestID != m.ActiveTargetRequest {
			return m, nil
		}
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Setup.ResumeReady = false
			m.Setup.ResumeMapPath = ""
			m.Setup.ResumeDetail = ""
			m.Page = PageChooseOutput
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityWarning}
			return m, nil
		}
		m.Setup.ResumeReady = typed.Status.CanResume
		m.Setup.ResumeMapPath = typed.Status.MapPath
		m.Setup.ResumeDetail = typed.Status.Detail
		if typed.Status.CanResume {
			m.Setup.ActionLabel = "Resume recovery"
			m.Notice = &NoticeModel{Text: firstNonEmpty(typed.Status.Detail, "A matching recovery can be resumed safely."), Severity: SeverityInfo}
		} else {
			m.Setup.ActionLabel = "Start a new recovery"
			m.Notice = nil
		}
		m.Page = PageReview
		m.Cursor = 0
		return m, nil
	case ResumableJobsDiscoveredMsg:
		if typed.RequestID != m.ActiveResumeRequest {
			return m, nil
		}
		if typed.Err != nil {
			m.LastError = typed.Err
			m.ResumeJobs = nil
			m.Page = PageChooseAction
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityWarning}
			return m, nil
		}
		m.ResumeJobs = append([]ResumableJobViewModel(nil), typed.Jobs...)
		m.Page = PageResumeJobs
		m.Cursor = 0
		if len(m.ResumeJobs) == 0 {
			m.Notice = &NoticeModel{Text: "No resumable recoveries were found in the current output folder.", Severity: SeverityInfo}
		} else {
			m.Notice = &NoticeModel{Text: "Choose a saved recovery to resume safely.", Severity: SeverityInfo}
		}
		return m, nil
	case JobStartedMsg:
		m.Page = PageRecovering
		m.Recovery.Status = typed.Status
		m.Recovery.OutputPath = typed.OutputPath
		m.Recovery.Phase = typed.Phase
		m.Recovery.TotalSectors = typed.TotalSectors
		m.Recovery.RecoveredSectors = 0
		m.Recovery.UnreadableSectors = 0
		m.Recovery.PausePending = false
		m.Details.Lines = buildRecoveryDetails(m)
		m.Notice = nil
		return m, nil
	case JobPausedMsg:
		m.Page = PagePaused
		m.Recovery.Status = "Recovery paused"
		m.Recovery.OutputPath = typed.OutputPath
		m.Recovery.RecoveredSectors = typed.RecoveredSectors
		m.Recovery.TotalSectors = typed.TotalSectors
		m.Recovery.UnreadableSectors = typed.UnreadableSectors
		m.Recovery.PausePending = false
		if typed.MapPath != "" {
			m.Setup.ResumeMapPath = typed.MapPath
		}
		m.Details.Lines = buildRecoveryDetails(m)
		m.Notice = &NoticeModel{Text: "Progress saved. Continue recovery when you are ready.", Severity: SeverityInfo}
		return m, nil
	case JobStartFailedMsg:
		m.LastError = typed.Err
		m.Page = PageChooseOutput
		if typed.Err != nil {
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityWarning}
		}
		return m, nil
	case ProgressMsg:
		m.Recovery.Phase = typed.Snapshot.Phase
		m.Recovery.RecoveredSectors = typed.Snapshot.RecoveredSectors
		m.Recovery.TotalSectors = typed.Snapshot.TotalSectors
		m.Recovery.UnreadableSectors = typed.Snapshot.UnreadableSectors
		m.Recovery.Status = typed.Snapshot.Status
		m.Recovery.Elapsed = typed.Snapshot.Elapsed
		m.Recovery.Remaining = typed.Snapshot.Remaining
		m.Recovery.ETA = typed.Snapshot.ETA
		m.Recovery.Throughput = typed.Snapshot.Throughput
		m.Recovery.LastIssue = append([]string(nil), typed.Snapshot.LastIssue...)
		m.Recovery.PausePending = typed.Snapshot.PausePending
		if typed.Snapshot.OutputPath != "" {
			m.Recovery.OutputPath = typed.Snapshot.OutputPath
		}
		m.Details.Lines = buildRecoveryDetails(m)
		m.Notice = nil
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
		if typed.Summary.ImagePath != "" {
			m.Recovery.OutputPath = typed.Summary.ImagePath
		}
		m.Recovery.RecoveredSectors = typed.Summary.RecoveredSectors
		m.Recovery.TotalSectors = typed.Summary.TotalSectors
		m.Recovery.UnreadableSectors = typed.Summary.UnresolvedSectors
		m.Details.Lines = buildSummaryDetails(m, typed)
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

	if m.Page == PageChooseOutput {
		return m.handleOutputPathInput(msg, key)
	}

	switch {
	case matchesKey(key, DefaultKeys().Force):
		switch m.Page {
		case PageRecovering, PagePaused:
			m.PreviousPage = m.Page
			m.Page = PageStopConfirm
			m.Cursor = 0
			return m, nil
		case PageStopConfirm:
			return m, stopAfterCheckpointEffect()
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
			m.Recovery.PausePending = true
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
		m.Page = PageChooseDrive
	case PageChooseOutput:
		if m.PreviousPage != 0 {
			m.Page, m.PreviousPage = m.PreviousPage, 0
		} else {
			m.Page = PageChooseAction
		}
	case PageReview:
		m.Page = PageChooseOutput
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
			if !m.MediaRecoverable {
				m.Notice = &NoticeModel{Text: firstNonEmpty(m.MediaRecoverabilityNote, "This disc cannot be recovered by the current build."), Severity: SeverityWarning}
				return m, nil
			}
			m.Setup.ActionLabel = "Start a new recovery"
			m.Setup.ResumeReady = false
			m.Setup.ResumeMapPath = ""
			m.Setup.ResumeDetail = ""
			if strings.TrimSpace(m.Setup.OutputPath) == "" || m.Setup.OutputPath == "Not chosen yet" {
				applyOutputPath(&m.Setup, "discrescue.iso")
			}
			m.PreviousPage = PageChooseAction
			m.Page = PageChooseOutput
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Edit the suggested path if you want a different folder or file name.", Severity: SeverityInfo}
			return m, nil
		case 1:
			if !m.MediaRecoverable {
				m.Notice = &NoticeModel{Text: firstNonEmpty(m.MediaRecoverabilityNote, "This disc cannot be recovered by the current build."), Severity: SeverityWarning}
				return m, nil
			}
			requestID := m.nextRequestID()
			m.ActiveResumeRequest = requestID
			m.ResumeJobs = nil
			m.Page = PageResumeJobs
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Checking the current output folder for resumable recoveries.", Severity: SeverityInfo}
			return m, findResumeJobsEffect(firstNonEmpty(strings.TrimSpace(m.Setup.OutputDirectory), "."), requestID)
		case 2:
			requestID := m.nextRequestID()
			m.ActiveHistoryRequest = requestID
			m.HistoryItems = nil
			m.Page = PageHistory
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Scanning the current output folder for processed media.", Severity: SeverityInfo}
			return m, browseHistoryEffect(firstNonEmpty(strings.TrimSpace(m.Setup.OutputDirectory), "."), requestID)
		case 3:
			m.Page = PageChooseDrive
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Choose one optical drive.", Severity: SeverityInfo}
			return m, nil
		default:
			return m, nil
		}
	case PageChooseOutput:
		requestID := m.nextRequestID()
		m.ActiveTargetRequest = requestID
		return m, inspectTargetEffect(m.Setup.OutputPath, requestID)
	case PageReview:
		switch m.Cursor {
		case 0:
			return m, startJobEffect()
		case 1:
			m.PreviousPage = PageReview
			m.Page = PageChooseOutput
			m.Cursor = 0
			return m, nil
		case 2:
			m.Page = PageChooseDrive
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Choose one optical drive.", Severity: SeverityInfo}
			return m, nil
		default:
			return m, nil
		}
	case PageResumeJobs:
		if len(m.ResumeJobs) == 0 || m.Cursor >= len(m.ResumeJobs) {
			m.Page = PageChooseAction
			m.Cursor = 0
			return m, nil
		}
		selected := m.ResumeJobs[m.Cursor]
		applyOutputPath(&m.Setup, selected.OutputPath)
		m.Setup.ResumeReady = true
		m.Setup.ResumeMapPath = selected.MapPath
		m.Setup.ResumeDetail = selected.Detail
		m.Setup.ActionLabel = "Resume recovery"
		m.Page = PageReview
		m.Cursor = 0
		m.Notice = &NoticeModel{Text: selected.Detail, Severity: SeverityInfo}
		return m, nil
	case PageHistory:
		if len(m.HistoryItems) == 0 || m.Cursor >= len(m.HistoryItems) {
			m.Page = PageChooseAction
			m.Cursor = 0
			return m, nil
		}
		selected := m.HistoryItems[m.Cursor]
		m.PreviousPage = PageHistory
		m.Details.Lines = buildProcessedMediaDetails(selected)
		m.Page = PageDetails
		return m, nil
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
			m.Page = PageChooseDrive
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Choose one optical drive.", Severity: SeverityInfo}
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
		return 4
	case PagePriorProcessing:
		return len(m.PriorView.Options)
	case PageReview:
		return 3
	case PageResumeJobs:
		if len(m.ResumeJobs) == 0 {
			return 1
		}
		return len(m.ResumeJobs) + 1
	case PageHistory:
		if len(m.HistoryItems) == 0 {
			return 1
		}
		return len(m.HistoryItems) + 1
	case PagePaused:
		return 2
	case PageStopConfirm:
		return 2
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
	m.Notice = &NoticeModel{Text: "Finding usable optical drives.", Severity: SeverityInfo}
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

func lookupHistoryEffect(basePath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectLookupHistory, BasePath: basePath, RequestID: requestID}
	}
}

func inspectTargetEffect(outputPath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectInspectTarget, OutputPath: outputPath, RequestID: requestID}
	}
}

func findResumeJobsEffect(basePath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectFindResumeJobs, BasePath: basePath, RequestID: requestID}
	}
}

func browseHistoryEffect(basePath string, requestID int) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectBrowseHistory, BasePath: basePath, RequestID: requestID}
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
		HistoryLine: "Checking this computer for matching saved work.",
	}
}

func buildProcessedMediaDetails(item ProcessedMediaViewModel) []string {
	lines := []string{
		"Image: " + item.ImagePath,
		"Status: " + item.Status,
	}
	if item.MapPath != "" {
		lines = append(lines, "Map: "+item.MapPath)
	}
	if item.ModifiedAt != "" {
		lines = append(lines, "Updated: "+item.ModifiedAt)
	}
	if item.Detail != "" {
		lines = append(lines, "Notes: "+item.Detail)
	}
	return lines
}

func (m Model) handleOutputPathInput(msg tea.KeyPressMsg, key string) (tea.Model, tea.Cmd) {
	switch {
	case matchesKey(key, DefaultKeys().Back):
		return m.handleBack()
	case key == "tab" || matchesKey(key, DefaultKeys().Down) || matchesKey(key, DefaultKeys().Up):
		if m.Setup.ActiveOutputField == OutputFieldDirectory {
			m.Setup.ActiveOutputField = OutputFieldFileName
		} else {
			m.Setup.ActiveOutputField = OutputFieldDirectory
		}
		return m, nil
	case matchesKey(key, DefaultKeys().Select):
		m.Setup.OutputDirectory = strings.TrimSpace(m.Setup.OutputDirectory)
		m.Setup.OutputFileName = strings.TrimSpace(m.Setup.OutputFileName)
		syncOutputPath(&m.Setup)
		if m.Setup.OutputDirectory == "" {
			m.Notice = &NoticeModel{Text: "Choose an output folder before continuing.", Severity: SeverityWarning}
			return m, nil
		}
		if m.Setup.OutputFileName == "" {
			m.Notice = &NoticeModel{Text: "Choose an output file name before continuing.", Severity: SeverityWarning}
			return m, nil
		}
		requestID := m.nextRequestID()
		m.ActiveTargetRequest = requestID
		m.Notice = &NoticeModel{Text: "Checking the selected output path.", Severity: SeverityInfo}
		return m, inspectTargetEffect(m.Setup.OutputPath, requestID)
	case key == "backspace" || key == "ctrl+h":
		trimLastOutputRune(&m.Setup)
		syncOutputPath(&m.Setup)
		clearResumeTargetState(&m.Setup)
		return m, nil
	default:
		if msg.Text != "" {
			appendOutputText(&m.Setup, msg.Text)
			syncOutputPath(&m.Setup)
			clearResumeTargetState(&m.Setup)
		}
		return m, nil
	}
}

func applyOutputPath(setup *JobSetupModel, fullPath string) {
	setup.OutputDirectory, setup.OutputFileName = splitOutputPath(fullPath)
	syncOutputPath(setup)
}

func splitOutputPath(fullPath string) (string, string) {
	fullPath = strings.TrimSpace(fullPath)
	if fullPath == "" || fullPath == "Not chosen yet" {
		return ".", ""
	}
	directory := filepath.Dir(fullPath)
	fileName := filepath.Base(fullPath)
	if directory == "" {
		directory = "."
	}
	if fileName == "." || fileName == string(filepath.Separator) {
		fileName = ""
	}
	return directory, fileName
}

func syncOutputPath(setup *JobSetupModel) {
	directory := strings.TrimSpace(setup.OutputDirectory)
	fileName := strings.TrimSpace(setup.OutputFileName)
	switch {
	case directory == "" && fileName == "":
		setup.OutputPath = "Not chosen yet"
	case directory == "":
		setup.OutputPath = fileName
	case fileName == "":
		setup.OutputPath = filepath.Clean(directory)
	default:
		setup.OutputPath = filepath.Join(directory, fileName)
	}
}

func clearResumeTargetState(setup *JobSetupModel) {
	setup.ResumeReady = false
	setup.ResumeMapPath = ""
	setup.ResumeDetail = ""
	setup.ActionLabel = "Start a new recovery"
}

func trimLastOutputRune(setup *JobSetupModel) {
	switch setup.ActiveOutputField {
	case OutputFieldDirectory:
		setup.OutputDirectory = trimLastRune(setup.OutputDirectory)
	default:
		setup.OutputFileName = trimLastRune(setup.OutputFileName)
	}
}

func appendOutputText(setup *JobSetupModel, value string) {
	switch setup.ActiveOutputField {
	case OutputFieldDirectory:
		setup.OutputDirectory += value
	default:
		setup.OutputFileName += value
	}
}

func trimLastRune(value string) string {
	if value == "" {
		return value
	}
	runes := []rune(value)
	return string(runes[:len(runes)-1])
}

func buildRecoveryDetails(m Model) []string {
	lines := []string{
		"Drive: " + firstNonEmpty(m.SelectedDrive.DisplayName, m.SelectedDrive.Path, "not selected"),
		"Media: " + firstNonEmpty(m.Identity.Detail, "not identified"),
		"Output: " + firstNonEmpty(m.Recovery.OutputPath, m.Setup.OutputPath),
		"Map: " + firstNonEmpty(m.Setup.ResumeMapPath, replaceExtension(firstNonEmpty(m.Recovery.OutputPath, m.Setup.OutputPath), ".drmap")),
		"Phase: " + firstNonEmpty(m.Recovery.Phase, "Waiting to start"),
		"Recovered sectors: " + formatCount(m.Recovery.RecoveredSectors),
		"Unreadable sectors: " + formatCount(m.Recovery.UnreadableSectors),
	}
	if m.Recovery.Remaining != "" {
		lines = append(lines, "Remaining: "+m.Recovery.Remaining)
	}
	if m.Recovery.Elapsed != "" {
		lines = append(lines, "Elapsed: "+m.Recovery.Elapsed)
	}
	if m.Recovery.ETA != "" {
		lines = append(lines, "ETA: "+m.Recovery.ETA)
	}
	if m.Recovery.Throughput != "" {
		lines = append(lines, "Rate: "+m.Recovery.Throughput)
	}
	lines = append(lines, "Status: "+recoveryDetailsStatusLine(m))
	lines = append(lines, m.Recovery.LastIssue...)
	return lines
}

func buildSummaryDetails(m Model, msg JobStoppedMsg) []string {
	lines := []string{
		"Drive: " + firstNonEmpty(m.SelectedDrive.DisplayName, m.SelectedDrive.Path, "not selected"),
		"Media: " + firstNonEmpty(m.Identity.Detail, "not identified"),
		"Image: " + msg.Summary.ImagePath,
	}
	if msg.Summary.MapPath != "" {
		lines = append(lines, "Map: "+msg.Summary.MapPath)
	}
	if msg.Err != nil {
		lines = append(lines, "Error: "+msg.Err.Error())
	} else if msg.Summary.NextAction != "" {
		lines = append(lines, "Next step: "+msg.Summary.NextAction)
	}
	return lines
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

func recoveryDetailsStatusLine(m Model) string {
	if m.Page == PageSummary && m.Summary.NextAction != "" {
		return m.Summary.NextAction
	}
	return firstNonEmpty(m.Recovery.Status, "Waiting to start")
}

func summarySecondaryActionLabel(m Model) string {
	if m.Recovery.UnreadableSectors > 0 {
		return "Retry unreadable sectors"
	}
	return "Verify an existing image"
}
