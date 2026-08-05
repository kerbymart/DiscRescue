package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) View() tea.View {
	if m.Width > 0 && m.Height > 0 && (m.Width < 40 || m.Height < 12) {
		view := tea.NewView("Window too small. Resize to at least 40x12.")
		view.WindowTitle = "DiscRescue"
		return view
	}

	content := renderPage(m)
	view := tea.NewView(content)
	view.WindowTitle = "DiscRescue"
	view.AltScreen = m.Page == PageRecovering || m.Page == PageDetails
	return view
}

func renderPage(m Model) string {
	width := contentWidth(m.Width)
	lines := []string{"DiscRescue", ""}
	lines = append(lines, wrapText(pageTitle(m.Page), width)...)
	lines = append(lines, "")
	lines = append(lines, renderPageBody(m, width)...)

	if m.Notice != nil && m.Notice.Text != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText("Status: "+m.Notice.Text, width)...)
	}

	lines = append(lines, "")
	lines = append(lines, renderFooter(m.Page, width))
	return strings.Join(lines, "\n") + "\n"
}

func pageTitle(page Page) string {
	switch page {
	case PageDiscover:
		return "Finding usable drives and resumable jobs"
	case PageChooseDrive:
		return "Choose a drive"
	case PagePriorProcessing:
		return "Matching contents and local history"
	case PageChooseAction:
		return "What do you want to do?"
	case PageChooseOutput:
		return "Choose output"
	case PageReview:
		return "Review and start"
	case PageRecovering:
		return "Recovery in progress"
	case PageSummary:
		return "Recovery summary"
	case PageResumeJobs:
		return "Resume unfinished recovery"
	case PageHistory:
		return "Browse processed media"
	case PageDetails:
		return "Recovery details"
	case PageAdvanced:
		return "Advanced settings"
	case PageAbout:
		return "About DiscRescue"
	default:
		return "DiscRescue"
	}
}

func renderPageBody(m Model, width int) []string {
	switch m.Page {
	case PageDiscover:
		return wrapText("One status sentence is shown here while discovery runs.", width)
	case PageChooseDrive:
		return renderDeviceList(m, width)
	case PagePriorProcessing:
		return renderPriorProcessing(m, width)
	case PageChooseAction:
		return renderActionList(m, width)
	case PageChooseOutput:
		return renderOutputPage(m, width)
	case PageReview:
		return renderReviewPage(m, width)
	case PageRecovering:
		return renderRecoveryPage(m, width)
	case PageSummary:
		return renderSummaryPage(m, width)
	case PageResumeJobs:
		return wrapText("Select a resumable job to continue safely.", width)
	case PageHistory:
		return wrapText("Browse local processed-media history in a one-column list.", width)
	case PageDetails:
		return renderDetailsPage(m, width)
	case PageAdvanced:
		return wrapText("Advanced settings stay separate from the normal setup flow.", width)
	case PageAbout:
		return wrapText("DiscRescue is a guided optical-disc recovery tool.", width)
	default:
		return nil
	}
}

func renderDeviceList(m Model, width int) []string {
	if len(m.Devices) == 0 {
		return wrapText("No usable optical drives found.", width)
	}
	lines := make([]string, 0, len(m.Devices))
	for i, device := range m.Devices {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, fitToWidth(prefix+device.DisplayName+" — "+device.Status, width))
	}
	return lines
}

func renderPriorProcessing(m Model, width int) []string {
	if m.PriorView.Kind == PriorProcessingNone || m.PriorView.Kind == PriorProcessingIndeterminate {
		return wrapText(m.PriorView.HistoryLine, width)
	}

	lines := append([]string{}, wrapText(m.PriorView.Title, width)...)
	for _, line := range m.PriorView.Body {
		lines = append(lines, "")
		lines = append(lines, wrapText(line, width)...)
	}
	if m.PriorView.ImagePath != "" {
		lines = append(lines, "")
		lines = append(lines, fitToWidth("Image   "+m.PriorView.ImagePath, width))
	}
	if m.PriorView.CopyLabel != "" {
		lines = append(lines, fitToWidth("Copy    "+m.PriorView.CopyLabel, width))
	}
	if m.PriorView.LastSaved != "" {
		lines = append(lines, fitToWidth("Last saved       "+m.PriorView.LastSaved, width))
	}
	if m.PriorView.Recovered != "" {
		lines = append(lines, fitToWidth("Recovered        "+m.PriorView.Recovered, width))
	}
	if m.PriorView.UnreadableSectors != "" {
		lines = append(lines, fitToWidth("Unreadable       "+m.PriorView.UnreadableSectors, width))
	}
	if len(m.PriorView.Options) > 0 {
		lines = append(lines, "")
		for i, option := range m.PriorView.Options {
			prefix := "  "
			if i == m.Cursor {
				prefix = "> "
			}
			lines = append(lines, fitToWidth(prefix+option, width))
		}
	}
	return lines
}

func renderActionList(m Model, width int) []string {
	actions := []string{
		"Start a new recovery",
		"Resume an unfinished recovery",
		"Verify an existing image",
		"Merge recovery captures",
		"Browse processed media",
	}
	lines := make([]string, 0, len(actions))
	for i, action := range actions {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, fitToWidth(prefix+action, width))
	}
	lines = append(lines, "")
	lines = append(lines, wrapText(m.PriorView.HistoryLine, width)...)
	if m.Identity.Detail != "" {
		lines = append(lines, fitToWidth("Disc: "+m.Identity.Detail, width))
	}
	return lines
}

func renderOutputPage(m Model, width int) []string {
	return []string{
		fitToWidth("Path    "+m.Setup.OutputPath, width),
		fitToWidth("Format  "+m.Setup.OutputFormat, width),
		fitToWidth("Space   "+m.Setup.FreeSpace, width),
	}
}

func renderReviewPage(m Model, width int) []string {
	lines := []string{
		fitToWidth("Drive   "+selectedDriveLabel(m), width),
		fitToWidth("Action  "+m.Setup.ActionLabel, width),
		fitToWidth("Output  "+m.Setup.OutputPath, width),
	}
	if m.Identity.Summary != "" {
		lines = append(lines, fitToWidth("Match   "+m.Identity.Summary, width))
	}
	return lines
}

func renderRecoveryPage(m Model, width int) []string {
	progress := "0%"
	if m.Recovery.TotalSectors > 0 {
		progress = fmt.Sprintf("%d%%", (m.Recovery.RecoveredSectors*100)/m.Recovery.TotalSectors)
	}
	return []string{
		fitToWidth("Phase      "+m.Recovery.Phase, width),
		fitToWidth("Recovered  "+progress, width),
		fitToWidth("Status     "+m.Recovery.Status, width),
	}
}

func renderSummaryPage(m Model, width int) []string {
	return []string{
		fitToWidth("Outcome  "+m.Recovery.Status, width),
	}
}

func renderDetailsPage(m Model, width int) []string {
	lines := make([]string, 0, len(m.Details.Lines))
	for _, line := range m.Details.Lines {
		lines = append(lines, wrapText(line, width)...)
	}
	return lines
}

func selectedDriveLabel(m Model) string {
	if m.Cursor >= 0 && m.Cursor < len(m.Devices) {
		return m.Devices[m.Cursor].DisplayName
	}
	if len(m.Devices) > 0 {
		return m.Devices[0].DisplayName
	}
	return "not selected"
}

func contentWidth(width int) int {
	if width <= 0 {
		return 76
	}
	value := width - 4
	if value < 40 {
		value = 40
	}
	if value > 76 {
		value = 76
	}
	return value
}

func renderFooter(page Page, width int) string {
	var footer string
	switch page {
	case PageRecovering:
		footer = "d details  q quit"
	case PageDetails:
		footer = "esc back  q quit"
	default:
		footer = "j/k move  enter select  esc back  q quit"
	}
	return fitToWidth(footer, width)
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return []string{""}
	}

	var lines []string
	var current strings.Builder

	for _, token := range strings.Fields(text) {
		if current.Len() == 0 {
			appendWrappedToken(&lines, &current, token, width)
			continue
		}

		if current.Len()+1+len(token) <= width {
			current.WriteByte(' ')
			current.WriteString(token)
			continue
		}

		lines = append(lines, current.String())
		current.Reset()
		appendWrappedToken(&lines, &current, token, width)
	}

	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func appendWrappedToken(lines *[]string, current *strings.Builder, token string, width int) {
	if len(token) <= width {
		current.WriteString(token)
		return
	}
	for len(token) > width {
		*lines = append(*lines, token[:width])
		token = token[width:]
	}
	current.WriteString(token)
}

func fitToWidth(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}
	return text[:width]
}
