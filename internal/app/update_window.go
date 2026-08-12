package app

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

func isLoadingPage(page Page) bool {
	switch page {
	case PageDiscover, PageInspectingMedia, PagePriorProcessing, PagePausing:
		return true
	default:
		return false
	}
}
func (m *Model) syncDetailsViewport() {
	m.DetailsViewport.SetContent(strings.Join(detailsLinesForView(*m), "\n"))
	m.DetailsViewport.GotoTop()
}

func (m Model) updateWindowMessage(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.Width = msg.Width
	m.Height = msg.Height
	componentWidth := interactiveWidth(msg.Width)
	inputWidth := componentWidth - 4
	if inputWidth < 12 {
		inputWidth = 12
	}
	m.DirectoryInput.SetWidth(inputWidth)
	m.FileNameInput.SetWidth(inputWidth)
	viewportHeight := layoutFor(msg.Width, msg.Height).Height - 4
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
}

func (m Model) updateSpinnerMessage(msg spinner.TickMsg) (tea.Model, tea.Cmd) {
	if !isLoadingPage(m.Page) {
		return m, nil
	}
	var cmd tea.Cmd
	m.LoadingSpinner, cmd = m.LoadingSpinner.Update(msg)
	return m, cmd
}
