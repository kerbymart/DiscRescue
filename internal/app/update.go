package app

import tea "charm.land/bubbletea/v2"

func (m Model) Init() tea.Cmd {
	return discoverDevicesEffect(m.ActiveDiscoveryRequest)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.updateMessage(msg)
}

func (m *Model) nextRequestID() int {
	requestID := m.NextRequestID
	if requestID == 0 {
		requestID = 1
	}
	m.NextRequestID = requestID + 1
	return requestID
}
