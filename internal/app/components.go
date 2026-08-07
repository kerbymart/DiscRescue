package app

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type driveListItem struct{ device DeviceSummary }

func (i driveListItem) Title() string { return i.device.DisplayName }
func (i driveListItem) Description() string {
	return fmt.Sprintf("%s · %s", i.device.Path, i.device.Status)
}
func (i driveListItem) FilterValue() string { return i.device.DisplayName + " " + i.device.Path }

type choiceListItem string

func (i choiceListItem) Title() string       { return string(i) }
func (i choiceListItem) Description() string { return "" }
func (i choiceListItem) FilterValue() string { return string(i) }

type detailListItem struct{ title, detail string }

func (i detailListItem) Title() string       { return i.title }
func (i detailListItem) Description() string { return i.detail }
func (i detailListItem) FilterValue() string { return i.title + " " + i.detail }

type compactDelegate struct{ list.DefaultDelegate }

func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	d.DefaultDelegate.Render(w, m, index, item)
}

func newCompactList(title string, items []list.Item, showDescription bool) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = showDescription
	delegate.SetSpacing(1)
	styles := delegate.Styles
	styles.NormalTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A7A3B8"))
	styles.NormalDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#686477"))
	styles.SelectedTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFDF5")).Background(lipgloss.Color("#6D28D9")).Padding(0, 1)
	styles.SelectedDesc = lipgloss.NewStyle().Foreground(lipgloss.Color("#E9D5FF")).Background(lipgloss.Color("#6D28D9")).Padding(0, 1)
	delegate.Styles = styles
	m := list.New(items, delegate, 0, 0)
	m.Title = title
	m.SetShowTitle(false)
	m.SetShowFilter(false)
	m.SetShowStatusBar(false)
	m.SetShowPagination(false)
	m.SetShowHelp(false)
	m.DisableQuitKeybindings()
	return m
}

func resizeCompactLists(width, availableHeight int, lists ...*list.Model) {
	if width < 1 {
		return
	}
	if availableHeight < 1 {
		availableHeight = 1
	}
	for _, component := range lists {
		if component == nil {
			continue
		}
		items := len(component.Items())
		if items == 0 {
			component.SetSize(width, 1)
			continue
		}
		// Each list owns its row spacing. Keep the component close to its
		// content so the shell does not turn a four-choice page into a wall
		// of empty terminal space.
		rowHeight := 1
		if described, ok := component.Items()[0].(interface{ Description() string }); ok && described.Description() != "" {
			rowHeight = 2
		}
		height := items*rowHeight + items
		if height > availableHeight {
			height = availableHeight
		}
		component.SetSize(width, height)
	}
}

func driveItems(devices []DeviceSummary) []list.Item {
	items := make([]list.Item, len(devices))
	for i, device := range devices {
		items[i] = driveListItem{device: device}
	}
	return items
}

func choiceItems(values []string) []list.Item {
	items := make([]list.Item, len(values))
	for i, value := range values {
		items[i] = choiceListItem(value)
	}
	return items
}

func resumeItems(jobs []ResumableJobViewModel) []list.Item {
	items := make([]list.Item, 0, len(jobs)+1)
	for _, job := range jobs {
		items = append(items, detailListItem{title: job.OutputPath, detail: job.Detail})
	}
	return append(items, choiceListItem("Back"))
}

func historyItems(items []ProcessedMediaViewModel) []list.Item {
	listItems := make([]list.Item, 0, len(items)+1)
	for _, item := range items {
		listItems = append(listItems, detailListItem{title: item.Title, detail: item.Status})
	}
	return append(listItems, choiceListItem("Back"))
}

func updateCompactList(m list.Model, msg tea.Msg) (list.Model, tea.Cmd) { return m.Update(msg) }
