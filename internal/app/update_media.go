package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) updateDevicesDiscovered(msg DevicesDiscoveredMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.ActiveDiscoveryRequest {
		return m, nil
	}
	if msg.Err != nil {
		m.Page = PageDiscoveryError
		m.setErrorNotice(contextDiscovery, msg.Err)
		return m, nil
	}
	previousSelection := m.SelectedDrive
	discoveredDevices := msg.Devices
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
}

func (m Model) updateMediaIdentified(msg MediaIdentifiedMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.ActiveMediaRequest {
		return m, nil
	}
	if msg.Err != nil {
		m.Page = PageInspectingMedia
		m.setErrorNotice(contextMedia, msg.Err)
		return m, nil
	}
	m.Identity = msg.Identity
	m.MediaFileSystem = msg.FileSystem
	m.MediaVolumeLabel = msg.VolumeLabel
	m.MediaLogicalSectorSize = msg.LogicalSectorSize
	m.MediaCapacitySectors = msg.CapacitySectors
	m.MediaRecoverable = msg.Recoverable
	m.MediaRecoverabilityNote = msg.RecoverabilityNote
	if msg.SuggestedOutputPath != "" {
		applyOutputPath(&m.Setup, msg.SuggestedOutputPath)
		m.syncOutputInputs()
		m.Setup.DefaultPath = msg.SuggestedOutputPath
	}
	m.Page = PageChooseAction
	m.Cursor = 0
	m.PriorView = defaultPriorProcessingView()
	m.PriorRecords = nil
	requestID := m.nextRequestID()
	m.ActiveLookupRequest = requestID
	if msg.Recoverable {
		m.Notice = &NoticeModel{Text: "Media inspection completed. Start a new recovery to create an image.", Severity: SeverityInfo}
	} else {
		m.Notice = &NoticeModel{Text: msg.RecoverabilityNote, Severity: SeverityWarning}
	}
	return m, lookupHistoryEffect(firstNonEmpty(strings.TrimSpace(m.Setup.OutputDirectory), "."), requestID)
}

func (m Model) updateRecoveryTargetInspected(msg RecoveryTargetInspectedMsg) (tea.Model, tea.Cmd) {
	if msg.RequestID != m.ActiveTargetRequest {
		return m, nil
	}
	if msg.Err != nil {
		m.Setup.ResumeReady = false
		m.Setup.ResumeMapPath = ""
		m.Setup.ResumeDetail = ""
		m.Setup.FreeSpace = "Unable to check free space for the selected target"
		m.Page = PageChooseOutput
		m.setErrorNotice(contextTarget, msg.Err)
		return m, nil
	}
	m.Setup.FreeSpace = describeTargetSpace(msg.Status)
	if !msg.Status.CanStartNew && !msg.Status.CanResume {
		m.Setup.ResumeReady = false
		m.Setup.ResumeMapPath = ""
		m.Setup.ResumeDetail = ""
		m.Setup.ActionLabel = "Start a new recovery"
		m.Page = PageChooseOutput
		if strings.TrimSpace(msg.Status.Detail) != "" {
			m.Notice = &NoticeModel{Text: msg.Status.Detail, Severity: SeverityWarning}
		} else {
			m.Notice = &NoticeModel{Text: "Choose a different output path before starting recovery.", Severity: SeverityWarning}
		}
		return m, nil
	}
	m.Setup.ResumeReady = msg.Status.CanResume
	m.Setup.ResumeMapPath = msg.Status.MapPath
	m.Setup.ResumeDetail = msg.Status.Detail
	if msg.Status.CanResume {
		m.Setup.ActionLabel = "Resume recovery"
		m.Notice = &NoticeModel{Text: firstNonEmpty(msg.Status.Detail, "A matching recovery can be resumed safely."), Severity: SeverityInfo}
	} else {
		m.Setup.ActionLabel = "Start a new recovery"
		m.Notice = &NoticeModel{Text: firstNonEmpty(msg.Status.Detail, "Use the suggested output target or edit it before starting recovery."), Severity: SeverityInfo}
	}
	m.Page = PageReview
	m.Cursor = 0
	return m, nil
}
