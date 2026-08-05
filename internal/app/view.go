package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m Model) View() tea.View {
	if m.Width > 0 && m.Height > 0 && (m.Width < 40 || m.Height < 12) {
		return tea.NewView("Window too small. Resize to at least 40x12.")
	}

	return tea.NewView(fmt.Sprintf(
		"DiscRescue\n\nInitial screen: %s\nFocus: %s\n\n%s\n%s\n",
		m.Screen,
		m.CurrentFocus,
		m.Notice,
		m.StatusLine,
	))
}
