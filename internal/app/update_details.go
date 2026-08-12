package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) updateDetailsKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.DetailsViewport.SetContent(strings.Join(detailsLinesForView(m), "\n"))
	m.DetailsViewport, cmd = m.DetailsViewport.Update(msg)
	return m, cmd
}
