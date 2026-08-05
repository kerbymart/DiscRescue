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
	case PagePaused:
		return "Recovery paused"
	case PageStopConfirm:
		return "Stop recovery?"
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
	case PagePaused:
		return renderPausedPage(m, width)
	case PageStopConfirm:
		return renderStopConfirmPage(m, width)
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
		fitToWidth("Drive       "+selectedDriveLabel(m), width),
		fitToWidth("Disc        "+discSummary(m), width),
		fitToWidth("Output      "+m.Setup.OutputPath, width),
		fitToWidth("Method      "+m.Setup.MethodLabel, width),
		fitToWidth("Copy label  "+m.Setup.CopyLabel, width),
	}
	lines = append(lines, "")
	options := []string{
		"Start recovery",
		"Change output",
		"Change method",
		"Add physical-copy label",
		"Advanced settings",
	}
	for i, option := range options {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, fitToWidth(prefix+option, width))
	}
	return lines
}

func renderRecoveryPage(m Model, width int) []string {
	progressBar := "[░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░]"
	progress := "0%"
	filled := 0
	if m.Recovery.TotalSectors > 0 {
		percent := (m.Recovery.RecoveredSectors * 100) / m.Recovery.TotalSectors
		progress = fmt.Sprintf("%d%%", percent)
		filled = int((m.Recovery.RecoveredSectors * 40) / m.Recovery.TotalSectors)
		if filled > 40 {
			filled = 40
		}
		progressBar = "[" + strings.Repeat("█", filled) + strings.Repeat("░", 40-filled) + "]"
	}
	lines := []string{
		fitToWidth(progressBar+" "+progress, width),
		"",
	}
	if m.Recovery.Phase != "" {
		lines = append(lines, fitToWidth(m.Recovery.Phase, width))
	}
	if m.Recovery.Remaining != "" {
		remaining := m.Recovery.Remaining
		if m.Recovery.ETA != "" {
			remaining += "  •  " + m.Recovery.ETA
		}
		lines = append(lines, fitToWidth(remaining, width))
	}
	lines = append(lines, "")
	lines = append(lines, fitToWidth(fmt.Sprintf("Recovered     %s sectors", formatCount(m.Recovery.RecoveredSectors)), width))
	lines = append(lines, fitToWidth(fmt.Sprintf("Unreadable    %s sectors", formatCount(m.Recovery.UnreadableSectors)), width))
	if len(m.Recovery.LastIssue) > 0 {
		lines = append(lines, "")
		for _, line := range m.Recovery.LastIssue {
			lines = append(lines, fitToWidth(line, width))
		}
	}
	return lines
}

func renderSummaryPage(m Model, width int) []string {
	lines := []string{fitToWidth(m.Recovery.Status, width), ""}
	if m.Recovery.UnreadableSectors == 0 {
		lines = append(lines,
			fitToWidth("Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath), width),
			fitToWidth(fmt.Sprintf("Recovered  %s of %s sectors", formatCount(m.Recovery.RecoveredSectors), formatCount(m.Recovery.TotalSectors)), width),
		)
		if m.Summary.Duration != "" {
			lines = append(lines, fitToWidth("Duration   "+m.Summary.Duration, width))
		}
	} else {
		lines = append(lines,
			fitToWidth(fmt.Sprintf("%s sectors could not be recovered.", formatCount(m.Recovery.UnreadableSectors)), width),
			fitToWidth("The image and map are complete enough to inspect or retry later.", width),
			"",
			fitToWidth("Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath), width),
			fitToWidth("Map        "+firstNonEmpty(m.Summary.MapPath, replaceExtension(m.Recovery.OutputPath, ".drmap")), width),
			fitToWidth("History    "+firstNonEmpty(m.Summary.CatalogStatus, "Recorded in local processed-media catalog"), width),
		)
	}
	lines = append(lines, "")
	for i, option := range summaryOptions(m) {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, fitToWidth(prefix+option, width))
	}
	return lines
}

func renderDetailsPage(m Model, width int) []string {
	lines := make([]string, 0, len(m.Details.Lines))
	for _, line := range m.Details.Lines {
		lines = append(lines, wrapText(line, width)...)
	}
	return lines
}

func renderPausedPage(m Model, width int) []string {
	lines := []string{
		fitToWidth("The current image and recovery map are safe to resume.", width),
	}
	if m.Recovery.PausePending {
		lines = append(lines, fitToWidth("Waiting for the current drive request to finish…", width))
	} else {
		lines = append(lines, fitToWidth("No new drive commands will be started while paused.", width))
	}
	lines = append(lines, "")
	options := []string{
		"Continue recovery",
		"Stop after checkpoint",
	}
	for i, option := range options {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, fitToWidth(prefix+option, width))
	}
	return lines
}

func renderStopConfirmPage(m Model, width int) []string {
	lines := []string{
		fitToWidth("The image can be resumed later.", width),
		"",
	}
	options := []string{
		"Save progress and stop",
		"Continue recovery",
		"Stop worker immediately",
	}
	for i, option := range options {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, fitToWidth(prefix+option, width))
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

func discSummary(m Model) string {
	if m.Identity.Detail != "" {
		return m.Identity.Detail
	}
	return "not identified"
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
		footer = "space pause  •  d details  •  q stop"
	case PagePaused:
		footer = "j/k select  •  enter choose  •  d details"
	case PageStopConfirm:
		footer = "j/k select  •  enter choose  •  esc continue"
	case PageDetails:
		footer = "esc back  •  up/down scroll"
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

func summaryOptions(m Model) []string {
	if m.Recovery.UnreadableSectors > 0 {
		return []string{
			"Retry unreadable sectors",
			"Exit and resume later",
			"View details",
		}
	}
	return []string{
		"Exit",
		"Verify image",
		"View details",
	}
}

func replaceExtension(path, extension string) string {
	if strings.HasSuffix(path, ".iso") {
		return strings.TrimSuffix(path, ".iso") + extension
	}
	return path + extension
}

func formatCount(value uint64) string {
	plain := fmt.Sprintf("%d", value)
	if len(plain) <= 3 {
		return plain
	}
	var groups []string
	for len(plain) > 3 {
		groups = append([]string{plain[len(plain)-3:]}, groups...)
		plain = plain[:len(plain)-3]
	}
	groups = append([]string{plain}, groups...)
	return strings.Join(groups, ",")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
