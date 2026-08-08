package app

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type driveListItem struct{ device DeviceSummary }

func (i driveListItem) Title() string { return i.device.DisplayName }
func (i driveListItem) Description() string {
	return fmt.Sprintf("%s · %s", i.device.Path, i.device.Status)
}
func (i driveListItem) FilterValue() string { return i.device.DisplayName + " " + i.device.Path }

type choiceListItem struct{ title, detail string }

func (i choiceListItem) Title() string       { return i.title }
func (i choiceListItem) Description() string { return i.detail }
func (i choiceListItem) FilterValue() string { return i.title + " " + i.detail }

type detailListItem struct{ title, detail string }

func (i detailListItem) Title() string       { return i.title }
func (i detailListItem) Description() string { return i.detail }
func (i detailListItem) FilterValue() string { return i.title + " " + i.detail }

type compactDelegate struct{ showDescription bool }

func (d compactDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	row, ok := item.(interface {
		Title() string
		Description() string
	})
	if !ok {
		return
	}

	selected := index == m.Index()
	marker := "  "
	if selected {
		marker = "> "
	}
	width := maxInt(12, m.Width())
	title := fitToWidth(marker+row.Title(), width-2)
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#B9B4C9")).Padding(0, 1)
	detailStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#716B82")).Padding(0, 1)
	if selected {
		titleStyle = titleStyle.Foreground(lipgloss.Color("#FFF8FF")).Background(lipgloss.Color("#6D28D9")).Bold(true).Width(width - 2)
		detailStyle = detailStyle.Foreground(lipgloss.Color("#E9D5FF")).Background(lipgloss.Color("#6D28D9")).Width(width - 2)
	}
	_, _ = io.WriteString(w, titleStyle.Render(title))
	if d.showDescription && strings.TrimSpace(row.Description()) != "" {
		detail := fitToWidth("  "+row.Description(), width-2)
		_, _ = io.WriteString(w, "\n"+detailStyle.Render(detail))
	}
}

func (d compactDelegate) Height() int {
	if d.showDescription {
		return 2
	}
	return 1
}

func (compactDelegate) Spacing() int                        { return 0 }
func (compactDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func newCompactList(title string, items []list.Item, showDescription bool) list.Model {
	delegate := compactDelegate{showDescription: showDescription}
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
		height := items * rowHeight
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

func recoveryActionItems() []list.Item {
	return []list.Item{
		choiceListItem{title: "Start a new recovery", detail: "Create a new image and durable recovery map."},
		choiceListItem{title: "Resume an unfinished recovery", detail: "Continue safely from matching saved work."},
		choiceListItem{title: "Browse processed media", detail: "Inspect images and maps in the output folder."},
		choiceListItem{title: "Choose another drive", detail: "Return to the optical-drive list."},
	}
}

func resumeItems(jobs []ResumableJobViewModel) []list.Item {
	items := make([]list.Item, 0, len(jobs)+1)
	for _, job := range jobs {
		items = append(items, detailListItem{title: job.OutputPath, detail: job.Detail})
	}
	return append(items, choiceListItem{title: "Back"})
}

func historyItems(items []ProcessedMediaViewModel) []list.Item {
	listItems := make([]list.Item, 0, len(items)+1)
	for _, item := range items {
		listItems = append(listItems, detailListItem{title: item.Title, detail: item.Status})
	}
	return append(listItems, choiceListItem{title: "Back"})
}

func updateCompactList(m list.Model, msg tea.Msg) (list.Model, tea.Cmd) { return m.Update(msg) }

func stylePathInput(input *textinput.Model) {
	styles := input.Styles()
	styles.Focused.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF8FF"))
	styles.Focused.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#716B82"))
	styles.Blurred.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#B9B4C9"))
	styles.Blurred.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#716B82"))
	styles.Cursor.Color = lipgloss.Color("#67E8F9")
	input.SetStyles(styles)
}
