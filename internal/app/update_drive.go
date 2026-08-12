package app

import (
	tea "charm.land/bubbletea/v2"

	"discrescue/internal/device"
)

func (m Model) ejectTargetDrive() DeviceSummary {
	if m.SelectedDrive.Path != "" {
		return m.SelectedDrive
	}
	if m.Cursor >= 0 && m.Cursor < len(m.Devices) {
		return m.Devices[m.Cursor]
	}
	return DeviceSummary{}
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

func (m Model) updateEjectCompleted(msg EjectCompletedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		if msg.Request.Mode == device.EjectNormal {
			m.setErrorNotice(contextEject, msg.Err)
		} else {
			m.setErrorNotice(contextForce, msg.Err)
		}
		if msg.Request.Mode == device.EjectNormal {
			m.PendingEject = device.EjectRequest{Mode: device.EjectForce, ExplicitConfirm: true}
			m.Page = PageEjectConfirm
			m.Cursor = 0
		} else {
			m.PendingEject = device.EjectRequest{}
			m.Page = PageChooseDrive
			m.Cursor = 0
		}
		return m, nil
	}
	m.PendingEject = device.EjectRequest{}
	m.PreserveEjectedDrive = true
	m.EjectedDrive = m.ejectTargetDrive()
	m.Notice = &NoticeModel{Text: firstNonEmpty(msg.Result.Detail, "Eject request accepted; refreshing drive state."), Severity: SeverityInfo}
	return m.beginDiscovery()
}
