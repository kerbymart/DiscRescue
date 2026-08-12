package app

import (
	tea "charm.land/bubbletea/v2"
	"strings"

	"discrescue/internal/device"
	"discrescue/internal/platform"
)

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())

	if m.Page == PageChooseOutput {
		return m.handleOutputPathInput(msg, key)
	}

	switch {
	case matchesKey(key, DefaultKeys().Force):
		switch m.Page {
		case PagePausing:
			if m.Recovery.ForceStopAvailable {
				m.Notice = &NoticeModel{Text: "Force-stopping the active device request; saved progress is preserved.", Severity: SeverityWarning}
				return m, stopImmediatelyEffect()
			}
			return m, nil
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
		if m.Page == PageRecovering || m.Page == PagePausing || m.Page == PagePaused || m.Page == PageSummary || m.noticeHasTechnicalDetail() {
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
			m.Page = PagePausing
			m.Recovery.PausePending = false
			m.Recovery.StopPending = true
			m.Recovery.ForceStopAvailable = false
			m.RestartLoadingSpinner = true
			m.Notice = &NoticeModel{Text: "Saving progress and stopping after the current drive request.", Severity: SeverityInfo}
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
		options := summaryOptions(m)
		if m.Cursor >= len(options) {
			m.Cursor = 0
		}
		switch options[m.Cursor] {
		case "Retry deferred sectors", "Retry unreadable sectors", "Retry unresolved sectors":
			if m.SelectedDrive.Path == "" || m.Setup.OutputPath == "" {
				m.Notice = &NoticeModel{Text: "Select the original drive and recovery output before retrying unresolved sectors.", Severity: SeverityWarning}
				return m, nil
			}
			m.Setup.Method = platform.RecoveryMethodBalanced
			m.Setup.MethodLabel = "Balanced recovery"
			m.Setup.ActionLabel = "Retry unresolved sectors"
			m.Setup.RetryUnresolved = true
			m.Setup.ResumeReady = true
			m.Setup.ResumeMapPath = firstNonEmpty(m.Summary.MapPath, replaceExtension(m.Setup.OutputPath, ".drmap"))
			m.Setup.ResumeDetail = "Retrying only unresolved sectors from the saved recovery map."
			m.Notice = &NoticeModel{Text: "Retrying unresolved sectors with a fresh bounded retry cycle.", Severity: SeverityInfo}
			return m, startJobEffect()
		case "Exit":
			return m, tea.Quit
		case "Choose another drive":
			m.Page = PageChooseDrive
			m.Cursor = 0
			m.Notice = &NoticeModel{Text: "Choose one optical drive.", Severity: SeverityInfo}
			return m, nil
		case "View details":
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
