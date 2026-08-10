package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"

	"discrescue/internal/device"
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
		componentWidth := interactiveWidth(typed.Width)
		inputWidth := componentWidth - 4
		if inputWidth < 12 {
			inputWidth = 12
		}
		m.DirectoryInput.SetWidth(inputWidth)
		m.FileNameInput.SetWidth(inputWidth)
		viewportHeight := layoutFor(typed.Width, typed.Height).Height - 4
		if viewportHeight < 1 {
			viewportHeight = 1
		}
		m.DetailsViewport.SetWidth(componentWidth)
		m.DetailsViewport.SetHeight(viewportHeight)
		listHeight := viewportHeight - 3
		if listHeight < 1 {
			listHeight = 1
		}
		if listHeight > 12 {
			listHeight = 12
		}
		resizeCompactLists(componentWidth, listHeight, &m.DriveList, &m.ActionList, &m.ResumeList, &m.HistoryList)
		return m, nil
	case tea.KeyPressMsg:
		if m.Page == PageChooseDrive && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Select) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Back) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Quit) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Refresh) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Eject) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().ForceEject) {
			var cmd tea.Cmd
			m.DriveList, cmd = updateCompactList(m.DriveList, typed)
			m.Cursor = m.DriveList.Index()
			return m, cmd
		}
		if m.Page == PageChooseAction && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Select) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Back) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Quit) {
			var cmd tea.Cmd
			m.ActionList, cmd = updateCompactList(m.ActionList, typed)
			m.Cursor = m.ActionList.Index()
			return m, cmd
		}
		if (m.Page == PageResumeJobs || m.Page == PageHistory) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Select) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Back) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Quit) {
			var cmd tea.Cmd
			if m.Page == PageResumeJobs {
				m.ResumeList, cmd = updateCompactList(m.ResumeList, typed)
				m.Cursor = m.ResumeList.Index()
			} else {
				m.HistoryList, cmd = updateCompactList(m.HistoryList, typed)
				m.Cursor = m.HistoryList.Index()
			}
			return m, cmd
		}
		if m.Page == PageDetails && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Back) && !matchesKey(strings.ToLower(typed.String()), DefaultKeys().Quit) {
			var cmd tea.Cmd
			m.DetailsViewport.SetContent(strings.Join(detailsLinesForView(m), "\n"))
			m.DetailsViewport, cmd = m.DetailsViewport.Update(typed)
			return m, cmd
		}
		return m.handleKeyPress(typed)
	case spinner.TickMsg:
		if !isLoadingPage(m.Page) {
			return m, nil
		}
		var cmd tea.Cmd
		m.LoadingSpinner, cmd = m.LoadingSpinner.Update(typed)
		return m, cmd
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
		previousSelection := m.SelectedDrive
		discoveredDevices := typed.Devices
		preserveEjectedDrive := m.PreserveEjectedDrive
		if preserveEjectedDrive && previousSelection.Path == "" {
			previousSelection = m.EjectedDrive
		}
		m.PreserveEjectedDrive = false
		m.EjectedDrive = DeviceSummary{}
		if len(discoveredDevices) == 0 && preserveEjectedDrive && previousSelection.Path != "" {
			drive := previousSelection
			drive.Status = "drive available; media ejected"
			discoveredDevices = []DeviceSummary{drive}
		}
		var mediaInvalidated bool
		m.Devices, m.SelectedDrive, mediaInvalidated = reconcileDevices(discoveredDevices, previousSelection)
		if preserveEjectedDrive {
			mediaInvalidated = true
		}
		if mediaInvalidated {
			m.ActiveMediaRequest = 0
			m.MediaFileSystem = ""
			m.MediaVolumeLabel = ""
			m.MediaLogicalSectorSize = 0
			m.MediaCapacitySectors = 0
			m.Identity = ContentIdentityViewModel{Summary: "Media changed; inspect the current disc before continuing."}
			m.PriorView = defaultPriorProcessingView()
		}
		m.DriveList.SetItems(driveItems(m.Devices))
		resizeCompactLists(interactiveWidth(m.Width), maxInt(1, m.Height-11), &m.DriveList)
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
			m.syncOutputInputs()
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
		m.ResumeJobs = append([]ResumableJobViewModel(nil), typed.Jobs...)
		m.ResumeList.SetItems(resumeItems(m.ResumeJobs))
		resizeCompactLists(interactiveWidth(m.Width), maxInt(1, m.Height-11), &m.ResumeList)
		if typed.View.Kind == PriorProcessingStrongResumable && len(typed.Jobs) > 0 {
			m.Page = PagePriorProcessing
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Matching recovery work was found on this computer.", Severity: SeverityInfo}
			return m, nil
		}
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
		m.HistoryList.SetItems(historyItems(m.HistoryItems))
		resizeCompactLists(interactiveWidth(m.Width), maxInt(1, m.Height-11), &m.HistoryList)
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
			m.Setup.FreeSpace = "Unable to check free space for the selected target"
			m.Page = PageChooseOutput
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityWarning}
			return m, nil
		}
		m.Setup.FreeSpace = describeTargetSpace(typed.Status)
		if !typed.Status.CanStartNew && !typed.Status.CanResume {
			m.Setup.ResumeReady = false
			m.Setup.ResumeMapPath = ""
			m.Setup.ResumeDetail = ""
			m.Setup.ActionLabel = "Start a new recovery"
			m.Page = PageChooseOutput
			if strings.TrimSpace(typed.Status.Detail) != "" {
				m.Notice = &NoticeModel{Text: typed.Status.Detail, Severity: SeverityWarning}
			} else {
				m.Notice = &NoticeModel{Text: "Choose a different output path before starting recovery.", Severity: SeverityWarning}
			}
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
			m.Notice = &NoticeModel{Text: firstNonEmpty(typed.Status.Detail, "Use the suggested output target or edit it before starting recovery."), Severity: SeverityInfo}
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
		m.Recovery.RecoveredSectors = typed.RecoveredSectors
		m.Recovery.UnreadableSectors = typed.UnreadableSectors
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
	case EjectCompletedMsg:
		if typed.Err != nil {
			m.LastError = typed.Err
			m.Notice = &NoticeModel{Text: typed.Err.Error(), Severity: SeverityWarning}
			if typed.Request.Mode == device.EjectNormal {
				m.PendingEject = device.EjectRequest{Mode: device.EjectForce, ExplicitConfirm: true}
				m.Page = PageEjectConfirm
				m.Cursor = 0
			}
			return m, nil
		}
		m.PendingEject = device.EjectRequest{}
		m.PreserveEjectedDrive = true
		m.EjectedDrive = m.ejectTargetDrive()
		m.Notice = &NoticeModel{Text: firstNonEmpty(typed.Result.Detail, "Eject request accepted; refreshing drive state."), Severity: SeverityInfo}
		return m.beginDiscovery()
	case EffectRequestedMsg:
		return m, nil
	}

	return m, nil
}

func isLoadingPage(page Page) bool {
	switch page {
	case PageDiscover, PageInspectingMedia, PagePriorProcessing, PagePausing:
		return true
	default:
		return false
	}
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())

	if m.Page == PageChooseOutput {
		return m.handleOutputPathInput(msg, key)
	}

	switch {
	case matchesKey(key, DefaultKeys().Force):
		switch m.Page {
		case PageRecovering, PagePausing, PagePaused:
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
		case PageRecovering, PagePausing, PagePaused:
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
	case matchesKey(key, DefaultKeys().Refresh):
		if m.Page == PageChooseDrive {
			return m.beginDiscovery()
		}
		return m, nil
	case matchesKey(key, DefaultKeys().Eject):
		if m.Page == PageChooseDrive {
			drive := m.ejectTargetDrive()
			if drive.Path != "" {
				return m, ejectEffect(drive.Path, device.EjectRequest{Mode: device.EjectNormal})
			}
		}
		return m, nil
	case matchesKey(key, DefaultKeys().ForceEject):
		if m.Page == PageChooseDrive && m.ejectTargetDrive().Path != "" {
			m.SelectedDrive = m.ejectTargetDrive()
			m.PendingEject = device.EjectRequest{Mode: device.EjectForce, ExplicitConfirm: true}
			m.Page = PageEjectConfirm
			m.Cursor = 0
			return m, nil
		}
		return m, nil
	case matchesKey(key, DefaultKeys().Up):
		m.moveCursor(-1)
		return m, nil
	case matchesKey(key, DefaultKeys().Down):
		m.moveCursor(1)
		return m, nil
	case matchesKey(key, DefaultKeys().Pause):
		switch m.Page {
		case PageRecovering:
			m.Page = PagePausing
			m.RestartLoadingSpinner = true
			m.Recovery.PausePending = true
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Pausing recovery after the current read completes.", Severity: SeverityInfo}
			return m, pauseJobEffect()
		case PagePaused:
			m.Page = PageRecovering
			m.Cursor = 0
			return m, resumeJobEffect()
		default:
			return m, nil
		}
	case matchesKey(key, DefaultKeys().Details):
		if m.Page == PageRecovering || m.Page == PagePausing || m.Page == PagePaused || m.Page == PageSummary {
			m.PreviousPage = m.Page
			m.Page = PageDetails
			m.syncDetailsViewport()
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

func (m Model) ejectTargetDrive() DeviceSummary {
	if m.SelectedDrive.Path != "" {
		return m.SelectedDrive
	}
	if m.Cursor >= 0 && m.Cursor < len(m.Devices) {
		return m.Devices[m.Cursor]
	}
	return DeviceSummary{}
}

func (m Model) handleBack() (tea.Model, tea.Cmd) {
	switch m.Page {
	case PageDetails, PageAdvanced, PageAbout, PageHistory, PageResumeJobs:
		if m.PreviousPage != 0 || m.Page == PageDetails {
			m.Page, m.PreviousPage = m.PreviousPage, 0
		}
	case PageStopConfirm:
		if m.PreviousPage == PagePaused || m.PreviousPage == PagePausing {
			m.Page = PagePaused
			if m.PreviousPage == PagePausing {
				m.Page = PagePausing
			}
		} else {
			m.Page = PageRecovering
		}
		m.Cursor = 0
	case PageEjectConfirm:
		m.Page = PageChooseDrive
		m.PendingEject = device.EjectRequest{}
		m.Cursor = 0
	case PageChooseAction:
		m.Page = PageChooseDrive
	case PageChooseOutput:
		if m.PreviousPage != 0 {
			m.Page, m.PreviousPage = m.PreviousPage, 0
		} else {
			m.Page = PageChooseAction
		}
		m.Setup.OutputEditing = false
	case PageReview:
		m.Page = PageChooseOutput
		m.Cursor = 2
		m.Setup.OutputEditing = false
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
		m.RestartLoadingSpinner = true
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
		m.RestartLoadingSpinner = true
		return m, identifyMediaEffect(m.SelectedDrive.Path, requestID)
	case PagePriorProcessing:
		switch m.PriorView.Kind {
		case PriorProcessingStrongCompleted, PriorProcessingStrongResumable, PriorProcessingProbable:
			switch m.Cursor {
			case 0:
				if m.PriorView.Kind == PriorProcessingStrongResumable && len(m.ResumeJobs) > 0 {
					if len(m.ResumeJobs) == 1 {
						selected := m.ResumeJobs[0]
						applyOutputPath(&m.Setup, selected.OutputPath)
						m.Setup.ResumeReady = true
						m.Setup.ResumeMapPath = selected.MapPath
						m.Setup.ResumeDetail = selected.Detail
						m.Setup.ActionLabel = "Resume recovery"
						m.Page = PageReview
						m.Cursor = 0
						m.Notice = &NoticeModel{Text: selected.Detail, Severity: SeverityInfo}
						return m, nil
					}
					m.Page = PageResumeJobs
					m.Cursor = 0
					m.Notice = &NoticeModel{Text: "Choose a saved recovery to resume safely.", Severity: SeverityInfo}
					return m, nil
				}
				m.Page = PageChooseAction
				m.Cursor = 0
				return m, nil
			case 1:
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
			m.Cursor = 2
			m.Setup.OutputEditing = false
			requestID := m.nextRequestID()
			m.ActiveTargetRequest = requestID
			m.Notice = &NoticeModel{Text: "Checking the suggested output path.", Severity: SeverityInfo}
			return m, inspectTargetEffect(m.Setup.OutputPath, requestID)
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
		if m.Cursor == 0 {
			m.Setup.ActiveOutputField = OutputFieldDirectory
			m.Setup.OutputEditing = true
			m.syncOutputInputs()
			m.DirectoryInput.Focus()
			m.Notice = &NoticeModel{Text: "Editing the output folder.", Severity: SeverityInfo}
			return m, nil
		}
		if m.Cursor == 1 {
			m.Setup.ActiveOutputField = OutputFieldFileName
			m.Setup.OutputEditing = true
			m.syncOutputInputs()
			m.FileNameInput.Focus()
			m.Notice = &NoticeModel{Text: "Editing the output file name.", Severity: SeverityInfo}
			return m, nil
		}
		requestID := m.nextRequestID()
		m.ActiveTargetRequest = requestID
		m.Setup.OutputEditing = false
		m.Notice = &NoticeModel{Text: "Checking the selected output path.", Severity: SeverityInfo}
		return m, inspectTargetEffect(m.Setup.OutputPath, requestID)
	case PageReview:
		switch m.Cursor {
		case 0:
			return m, startJobEffect()
		case 1:
			m.Setup.Method, m.Setup.MethodLabel = nextRecoveryMethod(m.Setup.Method)
			m.Notice = &NoticeModel{Text: "Recovery method changed to " + m.Setup.MethodLabel + ".", Severity: SeverityInfo}
			return m, nil
		case 2:
			m.PreviousPage = PageReview
			m.Page = PageChooseOutput
			m.Cursor = 2
			m.Setup.OutputEditing = false
			return m, nil
		case 3:
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
		m.syncDetailsViewport()
		return m, nil
	case PagePaused:
		switch m.Cursor {
		case 0:
			if m.Recovery.PausePending {
				m.Notice = &NoticeModel{Text: "Waiting for the current read to finish before recovery can continue.", Severity: SeverityInfo}
				return m, nil
			}
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
	case PagePausing:
		return m, nil
	case PageStopConfirm:
		switch m.Cursor {
		case 0:
			return m, stopAfterCheckpointEffect()
		case 1:
			if m.PreviousPage == PagePaused || m.PreviousPage == PagePausing {
				m.Page = PagePaused
				if m.PreviousPage == PagePausing {
					m.Page = PagePausing
				}
			} else {
				m.Page = PageRecovering
			}
			m.Cursor = 0
			return m, nil
		default:
			return m, nil
		}
	case PageEjectConfirm:
		if m.Cursor == 0 && m.SelectedDrive.Path != "" {
			return m, ejectEffect(m.SelectedDrive.Path, m.PendingEject)
		}
		m.Page = PageChooseDrive
		m.PendingEject = device.EjectRequest{}
		m.Cursor = 0
		return m, nil
	case PageSummary:
		switch m.Cursor {
		case 0:
			return m, tea.Quit
		case 1:
			m.Page = PageChooseDrive
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Choose one optical drive.", Severity: SeverityInfo}
			return m, nil
		case 2:
			m.PreviousPage = m.Page
			m.Page = PageDetails
			m.syncDetailsViewport()
			return m, nil
		default:
			return m, nil
		}
	default:
		return m, nil
	}
}

func (m *Model) syncDetailsViewport() {
	m.DetailsViewport.SetContent(strings.Join(detailsLinesForView(*m), "\n"))
	m.DetailsViewport.GotoTop()
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
		return 4
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
	case PageChooseOutput:
		return 3
	case PagePausing:
		return 0
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
	m.RestartLoadingSpinner = true
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

func ejectEffect(devicePath string, request device.EjectRequest) tea.Cmd {
	return func() tea.Msg {
		return EffectRequestedMsg{Kind: EffectEject, DevicePath: devicePath, Eject: request}
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
	if m.Setup.OutputEditing {
		if (m.DirectoryInput.Value() == "" || m.DirectoryInput.Value() == ".") && m.Setup.OutputDirectory != "" && m.Setup.OutputDirectory != "." {
			m.DirectoryInput.SetValue(m.Setup.OutputDirectory)
		}
		if m.FileNameInput.Value() == "" && m.Setup.OutputFileName != "" {
			m.FileNameInput.SetValue(m.Setup.OutputFileName)
		}
		if m.Setup.ActiveOutputField == OutputFieldDirectory {
			m.DirectoryInput.Focus()
		} else {
			m.FileNameInput.Focus()
		}
	}
	switch {
	case matchesKey(key, DefaultKeys().Back):
		if m.Setup.OutputEditing {
			m.Setup.OutputEditing = false
			m.blurOutputInputs()
			m.Notice = &NoticeModel{Text: "Stopped editing the output target.", Severity: SeverityInfo}
			return m, nil
		}
		return m.handleBack()
	case key == "tab":
		m.blurOutputInputs()
		if m.Setup.ActiveOutputField == OutputFieldDirectory {
			m.Setup.ActiveOutputField = OutputFieldFileName
		} else {
			m.Setup.ActiveOutputField = OutputFieldDirectory
		}
		if m.Setup.OutputEditing {
			if m.Setup.ActiveOutputField == OutputFieldDirectory {
				m.Cursor = 0
				m.DirectoryInput.Focus()
			} else {
				m.Cursor = 1
				m.FileNameInput.Focus()
			}
		}
		return m, nil
	case matchesKey(key, DefaultKeys().Up):
		if m.Setup.OutputEditing {
			return m, nil
		}
		m.moveCursor(-1)
		return m, nil
	case matchesKey(key, DefaultKeys().Down):
		if m.Setup.OutputEditing {
			return m, nil
		}
		m.moveCursor(1)
		return m, nil
	case matchesKey(key, DefaultKeys().Select):
		if !m.Setup.OutputEditing {
			return m.handleSelect()
		}
		m.syncOutputValues()
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
		m.Setup.OutputEditing = false
		m.blurOutputInputs()
		m.Notice = &NoticeModel{Text: "Finished editing the output target.", Severity: SeverityInfo}
		return m, nil
	default:
		if m.Setup.OutputEditing {
			var cmd tea.Cmd
			if m.Setup.ActiveOutputField == OutputFieldDirectory {
				m.DirectoryInput, cmd = m.DirectoryInput.Update(msg)
			} else {
				m.FileNameInput, cmd = m.FileNameInput.Update(msg)
			}
			m.syncOutputValues()
			clearResumeTargetState(&m.Setup)
			return m, cmd
		}
		return m, nil
	}
}

func (m *Model) syncOutputInputs() {
	m.DirectoryInput.SetValue(m.Setup.OutputDirectory)
	m.FileNameInput.SetValue(m.Setup.OutputFileName)
}

func (m *Model) syncOutputValues() {
	m.Setup.OutputDirectory = m.DirectoryInput.Value()
	m.Setup.OutputFileName = m.FileNameInput.Value()
	syncOutputPath(&m.Setup)
}

func (m *Model) blurOutputInputs() {
	m.DirectoryInput.Blur()
	m.FileNameInput.Blur()
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
	setup.FreeSpace = "Check the selected target to see free space and required size"
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

func nextRecoveryMethod(current platform.RecoveryMethod) (platform.RecoveryMethod, string) {
	switch current {
	case platform.RecoveryMethodBalanced:
		return platform.RecoveryMethodFast, "Fast recovery"
	case platform.RecoveryMethodFast:
		return platform.RecoveryMethodGentle, "Gentle recovery"
	default:
		return platform.RecoveryMethodBalanced, "Balanced recovery"
	}
}

func nextCopyLabel(current string) string {
	if current == "Not set (optional)" {
		return "Shelf B · Disc 14"
	}
	return "Not set (optional)"
}

func recoveryDetailsStatusLine(m Model) string {
	if m.Page == PagePausing || m.Recovery.PausePending {
		return "Pause requested. Waiting for the current drive request to finish safely."
	}
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

func describeTargetSpace(status platform.RecoveryTargetStatus) string {
	if !status.SpaceKnown {
		if status.RequiredBytes > 0 {
			return "Need " + humanBytes(status.RequiredBytes) + " for a full image; free space is unknown"
		}
		return "Free space is unknown for the selected target"
	}
	if status.RequiredBytes == 0 {
		return humanBytes(status.AvailableBytes) + " free on the selected drive"
	}
	if status.AvailableBytes >= status.RequiredBytes {
		return humanBytes(status.AvailableBytes) + " free; need " + humanBytes(status.RequiredBytes)
	}
	return humanBytes(status.AvailableBytes) + " free; need " + humanBytes(status.RequiredBytes) + " — choose another target"
}
