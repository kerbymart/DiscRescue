package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"discrescue/internal/buildinfo"
)

type layoutTier uint8

const (
	layoutFull layoutTier = iota
	layoutMedium
	layoutCompact
	layoutTooSmall
)

func (m Model) View() tea.View {
	tier := layoutTierFor(m.Width, m.Height)
	if tier == layoutTooSmall {
		lines := []string{
			"DiscRescue",
			"",
			"Window too small.",
			"Resize to at least 40x12.",
		}
		if m.Page == PageRecovering || m.Page == PagePaused || m.Page == PageStopConfirm || m.Page == PageDetails {
			lines = append(lines, "Recovery continues while you resize the window.")
		}
		view := tea.NewView(strings.Join(lines, "\n") + "\n")
		view.WindowTitle = "DiscRescue"
		view.AltScreen = usesAltScreen(m.Page)
		return view
	}

	content := renderPage(m, tier)
	view := tea.NewView(content)
	view.WindowTitle = "DiscRescue"
	view.AltScreen = usesAltScreen(m.Page)
	return view
}

func renderPage(m Model, tier layoutTier) string {
	width := contentWidth(m.Width)
	lines := []string{"DiscRescue", ""}
	lines = append(lines, wrapText(pageTitle(m.Page), width)...)
	lines = append(lines, "")
	lines = append(lines, renderPageBody(m, width, tier)...)

	if m.Notice != nil && m.Notice.Text != "" && tier != layoutCompact {
		lines = append(lines, "")
		lines = append(lines, wrapText("Status: "+m.Notice.Text, width)...)
	}

	lines = append(lines, "")
	lines = append(lines, renderFooter(m.Page, width, tier))
	return strings.Join(lines, "\n") + "\n"
}

func usesAltScreen(page Page) bool {
	return page == PageRecovering || page == PageDetails
}

func pageTitle(page Page) string {
	switch page {
	case PageDiscover:
		return "Finding usable optical drives"
	case PageNoDrives:
		return "No optical drives found"
	case PageDiscoveryError:
		return "Drive discovery needs attention"
	case PageChooseDrive:
		return "Choose a drive"
	case PageInspectingMedia:
		return "Inspecting selected media"
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

func renderPageBody(m Model, width int, tier layoutTier) []string {
	switch m.Page {
	case PageDiscover:
		return wrapText("Finding usable optical drives.", width)
	case PageNoDrives:
		return renderNoDrivesPage(width)
	case PageDiscoveryError:
		return renderDiscoveryErrorPage(m, width)
	case PageChooseDrive:
		return renderDeviceList(m, width)
	case PageInspectingMedia:
		return renderInspectingMediaPage(m, width)
	case PagePriorProcessing:
		return renderPriorProcessing(m, width, tier)
	case PageChooseAction:
		return renderActionList(m, width, tier)
	case PageChooseOutput:
		return renderOutputPage(m, width, tier)
	case PageReview:
		return renderReviewPage(m, width, tier)
	case PageRecovering:
		return renderRecoveryPage(m, width, tier)
	case PagePaused:
		return renderPausedPage(m, width)
	case PageStopConfirm:
		return renderStopConfirmPage(m, width)
	case PageSummary:
		return renderSummaryPage(m, width, tier)
	case PageResumeJobs:
		return wrapText("Select a resumable job to continue safely.", width)
	case PageHistory:
		return wrapText("Browse local processed-media history in a one-column list.", width)
	case PageDetails:
		return renderDetailsPage(m, width)
	case PageAdvanced:
		return wrapText("Advanced settings stay separate from the normal setup flow.", width)
	case PageAbout:
		return renderAboutPage(width)
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
		lines = append(lines, wrapText(prefix+device.DisplayName+" - "+device.Status, width)...)
	}
	return lines
}

func renderNoDrivesPage(width int) []string {
	lines := wrapText("No optical drives are currently available to DiscRescue.", width)
	lines = append(lines, "")
	lines = append(lines, wrapText("Press Enter to retry discovery or q to quit.", width)...)
	return lines
}

func renderDiscoveryErrorPage(m Model, width int) []string {
	text := "Drive discovery failed."
	if m.Notice != nil && m.Notice.Text != "" {
		text = m.Notice.Text
	}
	lines := wrapText(text, width)
	lines = append(lines, "")
	lines = append(lines, wrapText("Press Enter to retry discovery or q to quit.", width)...)
	return lines
}

func renderInspectingMediaPage(m Model, width int) []string {
	lines := wrapText("Inspecting the media in the selected drive.", width)
	if m.SelectedDrive.DisplayName != "" {
		lines = append(lines, "")
		lines = append(lines, labeledLines("Drive", m.SelectedDrive.DisplayName, width)...)
	}
	if m.SelectedDrive.Path != "" {
		lines = append(lines, labeledLines("Path", m.SelectedDrive.Path, width)...)
	}
	lines = append(lines, "")
	lines = append(lines, wrapText("Press Enter to retry or Esc to return to the drive list.", width)...)
	return lines
}

func renderPriorProcessing(m Model, width int, tier layoutTier) []string {
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
		lines = append(lines, labeledLines("Image", m.PriorView.ImagePath, width)...)
	}
	if tier != layoutCompact && m.PriorView.CopyLabel != "" {
		lines = append(lines, labeledLines("Copy", m.PriorView.CopyLabel, width)...)
	}
	if tier == layoutFull && m.PriorView.LastSaved != "" {
		lines = append(lines, labeledLines("Last saved", m.PriorView.LastSaved, width)...)
	}
	if tier == layoutFull && m.PriorView.Recovered != "" {
		lines = append(lines, labeledLines("Recovered", m.PriorView.Recovered, width)...)
	}
	if tier == layoutFull && m.PriorView.UnreadableSectors != "" {
		lines = append(lines, labeledLines("Unreadable", m.PriorView.UnreadableSectors, width)...)
	}
	if len(m.PriorView.Options) > 0 {
		lines = append(lines, "")
		for i, option := range m.PriorView.Options {
			prefix := "  "
			if i == m.Cursor {
				prefix = "> "
			}
			lines = append(lines, wrapText(prefix+option, width)...)
		}
	}
	return lines
}

func renderActionList(m Model, width int, tier layoutTier) []string {
	actions := []string{
		"Start a new recovery",
		"Choose another drive",
	}
	lines := make([]string, 0, len(actions))
	for i, action := range actions {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, wrapText(prefix+action, width)...)
	}
	if tier != layoutCompact {
		lines = append(lines, "")
		lines = append(lines, wrapText(m.PriorView.HistoryLine, width)...)
		if m.Identity.Detail != "" {
			lines = append(lines, wrapText("Disc: "+m.Identity.Detail, width)...)
		}
	}
	return lines
}

func renderOutputPage(m Model, width int, tier layoutTier) []string {
	path := m.Setup.OutputPath
	if path == "" {
		path = " "
	}
	path += "▌"
	lines := wrapText("Choose the output file path. Press Enter to continue with the current value, or edit it directly.", width)
	lines = append(lines, "")
	lines = append(lines, labeledLines("Path", path, width)...)
	if m.Setup.DefaultPath != "" && m.Setup.DefaultPath != "Not chosen yet" {
		lines = append(lines, labeledLines("Suggested", m.Setup.DefaultPath, width)...)
	}
	lines = append(lines, labeledLines("Format", m.Setup.OutputFormat, width)...)
	if tier != layoutCompact {
		lines = append(lines, labeledLines("Space", m.Setup.FreeSpace, width)...)
	}
	return lines
}

func renderReviewPage(m Model, width int, tier layoutTier) []string {
	lines := []string{}
	lines = append(lines, labeledLines("Drive", selectedDriveLabel(m), width)...)
	lines = append(lines, labeledLines("Disc", discSummary(m), width)...)
	lines = append(lines, labeledLines("Output", m.Setup.OutputPath, width)...)
	lines = append(lines, "")
	options := []string{
		"Start recovery",
		"Edit output path",
		"Choose another drive",
	}
	for i, option := range options {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, wrapText(prefix+option, width)...)
	}
	return lines
}

func renderRecoveryPage(m Model, width int, tier layoutTier) []string {
	progressBar := progressBarFor(m, tier)
	progress := "0%"
	if m.Recovery.TotalSectors > 0 {
		percent := (m.Recovery.RecoveredSectors * 100) / m.Recovery.TotalSectors
		progress = fmt.Sprintf("%d%%", percent)
	}
	lines := []string{
		fitToWidth(progressBar+" "+progress, width),
		"",
	}
	if m.Recovery.Phase != "" {
		lines = append(lines, fitToWidth(m.Recovery.Phase, width))
	}
	if m.Recovery.Status != "" {
		lines = append(lines, wrapText(m.Recovery.Status, width)...)
	}
	if m.Recovery.Remaining != "" {
		remaining := m.Recovery.Remaining
		if m.Recovery.ETA != "" && tier != layoutCompact {
			remaining += "  -  " + m.Recovery.ETA
		}
		if m.Recovery.Throughput != "" && tier != layoutCompact {
			remaining += "  -  " + m.Recovery.Throughput
		}
		lines = append(lines, wrapText(remaining, width)...)
	} else if m.Recovery.ETA == "" && tier == layoutFull {
		lines = append(lines, fitToWidth("Estimating time remaining...", width))
	}
	lines = append(lines, "")
	lines = append(lines, fitToWidth(fmt.Sprintf("Recovered     %s sectors", formatCount(m.Recovery.RecoveredSectors)), width))
	lines = append(lines, fitToWidth(fmt.Sprintf("Unreadable    %s sectors", formatCount(m.Recovery.UnreadableSectors)), width))
	if tier != layoutCompact && len(m.Recovery.LastIssue) > 0 {
		lines = append(lines, "")
		for _, line := range m.Recovery.LastIssue {
			lines = append(lines, wrapText(line, width)...)
		}
	}
	return lines
}

func renderSummaryPage(m Model, width int, tier layoutTier) []string {
	lines := []string{fitToWidth(m.Recovery.Status, width), ""}
	if m.Recovery.UnreadableSectors == 0 {
		lines = append(lines,
			labeledLines("Image", firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath), width)...,
		)
		lines = append(lines,
			fitToWidth(fmt.Sprintf("Recovered  %s of %s sectors", formatCount(m.Recovery.RecoveredSectors), formatCount(m.Recovery.TotalSectors)), width),
		)
		if tier != layoutCompact && m.Summary.Duration != "" {
			lines = append(lines, fitToWidth("Duration   "+m.Summary.Duration, width))
		}
	} else {
		lines = append(lines,
			fitToWidth(fmt.Sprintf("%s sectors could not be recovered.", formatCount(m.Recovery.UnreadableSectors)), width),
			fitToWidth("The image and map are complete enough to inspect or retry later.", width),
			"",
		)
		lines = append(lines, labeledLines("Image", firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath), width)...)
		if tier != layoutCompact {
			lines = append(lines, labeledLines("Map", firstNonEmpty(m.Summary.MapPath, replaceExtension(m.Recovery.OutputPath, ".drmap")), width)...)
			lines = append(lines, labeledLines("History", firstNonEmpty(m.Summary.CatalogStatus, "Recorded in local processed-media catalog"), width)...)
		}
	}
	lines = append(lines, "")
	for i, option := range summaryOptions(m) {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, wrapText(prefix+option, width)...)
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
		lines = append(lines, fitToWidth("Waiting for the current drive request to finish...", width))
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
		lines = append(lines, wrapText(prefix+option, width)...)
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
		lines = append(lines, wrapText(prefix+option, width)...)
	}
	return lines
}

func renderAboutPage(width int) []string {
	lines := []string{
		"DiscRescue is a guided optical-disc recovery tool.",
		"",
		"Version    " + buildinfo.Version,
		"Commit     " + buildinfo.Commit,
		"Build date " + buildinfo.BuildDate,
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, wrapText(line, width)...)
	}
	return out
}

func selectedDriveLabel(m Model) string {
	if strings.TrimSpace(m.SelectedDrive.DisplayName) != "" {
		return m.SelectedDrive.DisplayName
	}
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

func layoutTierFor(width, height int) layoutTier {
	if width > 0 && height > 0 {
		if width < 40 || height < 12 {
			return layoutTooSmall
		}
		if width < 60 || height < 18 {
			return layoutCompact
		}
		if width < 80 || height < 24 {
			return layoutMedium
		}
	}
	return layoutFull
}

func contentWidth(width int) int {
	if width <= 0 {
		return 76
	}
	value := width - 4
	if value < 24 {
		value = 24
	}
	if value > 76 {
		value = 76
	}
	return value
}

func renderFooter(page Page, width int, tier layoutTier) string {
	var footer string
	switch page {
	case PageChooseOutput:
		if tier == layoutCompact {
			footer = "type path  enter continue  esc back"
		} else {
			footer = "type path  -  enter continue  -  backspace delete  -  esc back"
		}
	case PageRecovering:
		if tier == layoutCompact {
			footer = "space pause  d details"
		} else {
			footer = "space pause  -  d details  -  q stop"
		}
	case PagePaused:
		footer = "j/k select  -  enter choose  -  d details"
	case PageStopConfirm:
		footer = "j/k select  -  enter choose  -  esc continue"
	case PageDetails:
		footer = "esc back  -  up/down scroll"
	case PageNoDrives, PageDiscoveryError, PageInspectingMedia:
		footer = "enter retry  -  esc back  -  q quit"
	default:
		if tier == layoutCompact {
			footer = "enter select  esc back"
		} else {
			footer = "j/k move  enter select  esc back  q quit"
		}
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

func labeledLines(label, value string, width int) []string {
	prefix := label
	if len(prefix) < 11 {
		prefix += strings.Repeat(" ", 11-len(prefix))
	}
	return wrapTextWithPrefix(prefix, value, width)
}

func wrapTextWithPrefix(prefix, value string, width int) []string {
	if width <= len(prefix)+1 {
		return append([]string{prefix}, wrapText(value, width)...)
	}
	available := width - len(prefix) - 1
	if available < 8 {
		available = 8
	}
	wrapped := wrapText(value, available)
	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		if i == 0 {
			lines = append(lines, prefix+" "+line)
			continue
		}
		lines = append(lines, strings.Repeat(" ", len(prefix)+1)+line)
	}
	return lines
}

func fitToWidth(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}
	return text[:width]
}

func summaryOptions(m Model) []string {
	return []string{
		"Exit",
		"Choose another drive",
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

func progressBarFor(m Model, tier layoutTier) string {
	width := 40
	if tier == layoutMedium {
		width = 28
	}
	if tier == layoutCompact {
		width = 16
	}
	if width < 8 {
		width = 8
	}

	filled := 0
	if m.Recovery.TotalSectors > 0 {
		filled = int((m.Recovery.RecoveredSectors * uint64(width)) / m.Recovery.TotalSectors)
		if filled > width {
			filled = width
		}
	}

	filledGlyph := "#"
	emptyGlyph := "."
	if !m.Monochrome && tier != layoutCompact {
		filledGlyph = "█"
		emptyGlyph = "░"
	}
	return "[" + strings.Repeat(filledGlyph, filled) + strings.Repeat(emptyGlyph, width-filled) + "]"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
