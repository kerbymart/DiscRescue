package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func matchesKey(key string, options []string) bool {
	for _, option := range options {
		if key == option {
			return true
		}
	}
	return false
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
	case PageEjectConfirm:
		return 2
	case PageSummary:
		return len(summaryOptions(m))
	default:
		return 0
	}
}

func (m Model) updateKeyMessage(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := strings.ToLower(msg.String())
	if m.Page == PageChooseDrive && !matchesKey(key, DefaultKeys().Select) && !matchesKey(key, DefaultKeys().Back) && !matchesKey(key, DefaultKeys().Quit) && !matchesKey(key, DefaultKeys().Refresh) && !matchesKey(key, DefaultKeys().Eject) && !matchesKey(key, DefaultKeys().ForceEject) {
		var cmd tea.Cmd
		m.DriveList, cmd = updateCompactList(m.DriveList, msg)
		m.Cursor = m.DriveList.Index()
		return m, cmd
	}
	if m.Page == PageChooseAction && !matchesKey(key, DefaultKeys().Select) && !matchesKey(key, DefaultKeys().Back) && !matchesKey(key, DefaultKeys().Quit) {
		var cmd tea.Cmd
		m.ActionList, cmd = updateCompactList(m.ActionList, msg)
		m.Cursor = m.ActionList.Index()
		return m, cmd
	}
	if (m.Page == PageResumeJobs || m.Page == PageHistory) && !matchesKey(key, DefaultKeys().Select) && !matchesKey(key, DefaultKeys().Back) && !matchesKey(key, DefaultKeys().Quit) {
		var cmd tea.Cmd
		if m.Page == PageResumeJobs {
			m.ResumeList, cmd = updateCompactList(m.ResumeList, msg)
			m.Cursor = m.ResumeList.Index()
		} else {
			m.HistoryList, cmd = updateCompactList(m.HistoryList, msg)
			m.Cursor = m.HistoryList.Index()
		}
		return m, cmd
	}
	if m.Page == PageDetails && !matchesKey(key, DefaultKeys().Back) && !matchesKey(key, DefaultKeys().Quit) {
		return m.updateDetailsKey(msg)
	}
	return m.handleKeyPress(msg)
}
