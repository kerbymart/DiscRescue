package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = typed.Width
		m.Height = typed.Height
		return m, nil
	case tea.KeyPressMsg:
		switch strings.ToLower(typed.String()) {
		case DefaultKeys().Quit:
			m.ShouldQuit = true
			return m, tea.Quit
		case DefaultKeys().Start:
			m.Notice = "Bootstrap complete. Continue with simulator and coordinator work."
			return m, nil
		}
	case WorkerStatusReceived:
		m.StatusLine = typed.Status
		return m, nil
	}

	return m, nil
}
