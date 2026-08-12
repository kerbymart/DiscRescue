package app

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

func recoveryOutputName(m Model) string {
	path := firstNonEmpty(m.Recovery.OutputPath, m.Setup.OutputPath)
	if path == "" || path == "Not chosen yet" {
		return ""
	}
	path = strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func recoveryTimeSummary(m Model, tier layoutTier) string {
	remaining := strings.TrimSpace(m.Recovery.Remaining)
	eta := strings.TrimSpace(m.Recovery.ETA)

	if tier == layoutCompact {
		return remaining
	}

	if remaining == "" && eta == "" {
		if tier == layoutFull {
			return "Estimating time remaining..."
		}
		return ""
	}
	if remaining == "" {
		return eta
	}
	if eta == "" {
		if tier == layoutFull {
			return remaining + "  ETA estimating..."
		}
		return remaining
	}
	return remaining + "  ETA " + eta
}

func renderRecoveryPage(m Model, width int, tier layoutTier) []string {
	return renderRecoveryDashboard(m, width, tier)
}
func renderRecoveryDashboard(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	phase := firstNonEmpty(m.Recovery.Phase, "Preparing recovery")
	status := firstNonEmpty(m.Recovery.Status, "Reading sectors from the selected optical drive.")
	if tier == layoutCompact || tier == layoutMedium {
		if tier == layoutCompact {
			lines := []string{theme.Accent.Render(phase), recoveryProgressLine(m, width, tier)}
			lines = append(lines, recoveryCompactMetricRows(theme, m)...)
			if summary := recoveryTimeSummary(m, tier); summary != "" {
				lines = append(lines, summary)
			}
			return lines
		}
		lines := []string{theme.Accent.Render(phase), fitToWidth(theme.Muted.Render(status), width), recoveryProgressLine(m, width, tier)}
		lines = append(lines, recoveryMetricRows(theme, m)...)
		if summary := recoveryTimeSummary(m, tier); summary != "" {
			lines = append(lines, summary)
		}
		if tier != layoutCompact && m.Height >= 22 && (m.Recovery.Throughput != "" || m.Recovery.Elapsed != "") {
			lines = append(lines, "Rate  "+firstNonEmpty(m.Recovery.Throughput, "—")+"    Elapsed  "+firstNonEmpty(m.Recovery.Elapsed, "—"))
		}
		if tier != layoutCompact && m.Height >= 24 && len(m.Recovery.LastIssue) > 0 {
			lines = append(lines, "Last issue  "+m.Recovery.LastIssue[0])
		}
		return lines
	}

	if m.Height < 28 {
		phaseLine := theme.Badge.Render(phase) + "  " + theme.Muted.Render(status)
		lines := []string{fitToWidth(phaseLine, width)}
		lines = append(lines, recoveryDashboardPanel(theme, m, width, true)...)
		return lines
	}

	lines := []string{theme.Badge.Render(phase), theme.Muted.Render(status), ""}
	lines = append(lines, recoveryDashboardPanel(theme, m, width, false)...)
	if len(m.Recovery.LastIssue) > 0 {
		lines = append(lines, "", theme.Divider.Render(strings.Repeat("─", maxInt(1, width))))
		lines = append(lines, theme.Label.Render("LAST ISSUE")+"  "+fitToWidth(m.Recovery.LastIssue[0], maxInt(12, width-14)))
	}
	return lines
}
func recoveryDashboardPanel(theme Theme, m Model, width int, dense bool) []string {
	if width < 32 {
		return []string{recoveryProgressLine(m, width, layoutCompact)}
	}
	border := func(value string) string { return theme.Divider.Render(value) }
	outerWidth := width
	innerWidth := outerWidth - 4
	covered := recoveryCoveredSectors(m)
	percent := 0
	if m.Recovery.TotalSectors > 0 {
		percent = int((covered * 100) / m.Recovery.TotalSectors)
		if percent > 100 {
			percent = 100
		}
	}

	primaryWidths := distributedCellWidths(outerWidth, 4)
	timingWidths := distributedCellWidths(outerWidth, 2)
	remaining := firstNonEmpty(m.Recovery.Remaining, "—")
	if dense {
		remaining = compactRecoveryRemaining(remaining)
	}
	eta := firstNonEmpty(m.Recovery.ETA, "estimating...")
	primaryLabels := []string{"Recovered", "Deferred", "Unreadable", "Remaining"}
	primaryValues := []string{
		formatCount(m.Recovery.RecoveredSectors) + " sectors",
		formatCount(m.Recovery.DeferredSectors) + " sectors",
		formatCount(m.Recovery.UnreadableSectors) + " sectors",
		remaining,
	}
	if dense {
		primaryValues[0] = formatCount(m.Recovery.RecoveredSectors)
		primaryValues[1] = formatCount(m.Recovery.DeferredSectors)
		primaryValues[2] = formatCount(m.Recovery.UnreadableSectors)
	}

	lines := []string{
		border("╭" + strings.Repeat("─", outerWidth-2) + "╮"),
		recoveryPanelContentLine(theme, joinEdges(theme.AccentSoft.Render("Coverage"), theme.AccentSoft.Render(fmt.Sprintf("%d%%", percent)), innerWidth), outerWidth),
		recoveryPanelContentLine(theme, segmentedRecoveryBar(m, innerWidth), outerWidth),
		recoveryPanelContentLine(theme, theme.AccentSoft.Render(fmt.Sprintf("%s / %s sectors", formatCount(covered), formatCount(m.Recovery.TotalSectors))), outerWidth),
		recoveryPanelSectionDivider(theme, outerWidth, nil, primaryWidths),
		recoveryPanelCellLine(theme, primaryWidths, primaryLabels, theme.Accent),
		recoveryPanelCellLine(theme, primaryWidths, primaryValues, theme.Text),
	}
	etaDetail := "ETA " + eta
	if dense && m.Recovery.ETA != "" {
		etaDetail = eta
	} else if dense {
		etaDetail = "ETA estimating"
	}
	lines = append(lines, recoveryPanelCellLine(theme, primaryWidths, []string{"", "", "", etaDetail}, theme.AccentSoft))
	lines = append(lines,
		recoveryPanelSectionDivider(theme, outerWidth, primaryWidths, timingWidths),
		recoveryPanelCellLine(theme, timingWidths, []string{"Rate", "Elapsed"}, theme.Accent),
		recoveryPanelCellLine(theme, timingWidths, []string{firstNonEmpty(m.Recovery.Throughput, "—"), firstNonEmpty(m.Recovery.Elapsed, "—")}, theme.Text),
		border("╰"+strings.Repeat("─", outerWidth-2)+"╯"),
	)
	return lines
}
func compactRecoveryRemaining(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "—"
	}
	if fields := strings.Fields(value); len(fields) >= 2 {
		return strings.Join(fields[:2], " ")
	}
	return value
}
func distributedCellWidths(outerWidth, columns int) []int {
	available := outerWidth - 2 - (columns - 1)
	widths := make([]int, columns)
	for i := range widths {
		widths[i] = available / columns
		if i >= columns-available%columns {
			widths[i]++
		}
	}
	return widths
}
func recoveryPanelContentLine(theme Theme, content string, outerWidth int) string {
	innerWidth := outerWidth - 4
	content = fitToWidth(content, innerWidth)
	padding := strings.Repeat(" ", maxInt(0, innerWidth-lipgloss.Width(content)))
	return theme.Divider.Render("│") + " " + content + padding + " " + theme.Divider.Render("│")
}
func recoveryPanelCellLine(theme Theme, widths []int, values []string, style lipgloss.Style) string {
	var line strings.Builder
	line.WriteString(theme.Divider.Render("│"))
	for i, width := range widths {
		value := ""
		if i < len(values) {
			value = values[i]
		}
		innerWidth := maxInt(0, width-2)
		value = fitToWidth(style.Render(value), innerWidth)
		line.WriteString(" ")
		line.WriteString(value)
		line.WriteString(strings.Repeat(" ", maxInt(0, innerWidth-lipgloss.Width(value))))
		line.WriteString(" ")
		if i < len(widths)-1 {
			line.WriteString(theme.Divider.Render("│"))
		}
	}
	line.WriteString(theme.Divider.Render("│"))
	return line.String()
}
func recoveryPanelSectionDivider(theme Theme, outerWidth int, upperWidths, lowerWidths []int) string {
	interior := []rune(strings.Repeat("─", outerWidth-2))
	for _, position := range recoveryPanelBoundaries(upperWidths) {
		if position >= 0 && position < len(interior) {
			interior[position] = '┴'
		}
	}
	for _, position := range recoveryPanelBoundaries(lowerWidths) {
		if position >= 0 && position < len(interior) {
			if interior[position] == '┴' {
				interior[position] = '┼'
			} else {
				interior[position] = '┬'
			}
		}
	}
	return theme.Divider.Render("├" + string(interior) + "┤")
}
func recoveryPanelBoundaries(widths []int) []int {
	if len(widths) < 2 {
		return nil
	}
	boundaries := make([]int, 0, len(widths)-1)
	position := 0
	for _, width := range widths[:len(widths)-1] {
		position += width
		boundaries = append(boundaries, position)
		position++
	}
	return boundaries
}
func recoveryProgressLine(m Model, width int, tier layoutTier) string {
	covered := recoveryCoveredSectors(m)
	percent := 0
	if m.Recovery.TotalSectors > 0 {
		percent = int((covered * 100) / m.Recovery.TotalSectors)
		if percent > 100 {
			percent = 100
		}
	}
	barWidth := width - 2
	if tier == layoutCompact {
		if barWidth > 24 {
			barWidth = 24
		}
	}
	if barWidth < 8 {
		barWidth = 8
	}
	bar := segmentedRecoveryBar(m, barWidth)
	label := fmt.Sprintf("Coverage  %d%%   %s / %s sectors", percent, formatCount(covered), formatCount(m.Recovery.TotalSectors))
	return fitToWidth(bar, width) + "\n" + fitToWidth(label, width)
}
func recoveryMetricRows(theme Theme, m Model) []string {
	metrics := []struct {
		label string
		value string
		style lipgloss.Style
	}{
		{label: "Recovered", value: formatCount(m.Recovery.RecoveredSectors), style: theme.Accent},
		{label: "Deferred", value: formatCount(m.Recovery.DeferredSectors), style: theme.AccentSoft},
		{label: "Unreadable", value: formatCount(m.Recovery.UnreadableSectors), style: theme.Muted},
	}
	lines := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		lines = append(lines, metric.style.Width(11).Render(metric.label)+theme.Text.Render(metric.value+" sectors"))
	}
	return lines
}
func recoveryCompactMetricRows(theme Theme, m Model) []string {
	return []string{
		theme.Accent.Width(11).Render("Recovered") + theme.Text.Render(formatCount(m.Recovery.RecoveredSectors)+" sectors"),
		theme.AccentSoft.Width(11).Render("Deferred") + theme.Text.Render(formatCount(m.Recovery.DeferredSectors)+"  unreadable "+formatCount(m.Recovery.UnreadableSectors)),
	}
}
func segmentedRecoveryBar(m Model, width int) string {
	if width < 1 {
		return ""
	}
	counts := recoveryDisplayCounts(m)
	cells := allocateRecoveryBarCells(width, counts, m.Recovery.TotalSectors)
	if m.Monochrome {
		return "[" + strings.Repeat("=", cells[0]) + strings.Repeat("~", cells[1]) + strings.Repeat("x", cells[2]) + strings.Repeat(".", cells[3]) + "]"
	}
	theme := newTheme(false, m.DarkBackground)
	recovered := theme.RecoveryRecovered.Render(strings.Repeat("█", cells[0]))
	deferred := theme.RecoveryDeferred.Render(strings.Repeat("█", cells[1]))
	unreadable := theme.RecoveryUnreadable.Render(strings.Repeat("█", cells[2]))
	pending := theme.RecoveryPending.Render(strings.Repeat("█", cells[3]))
	return recovered + deferred + unreadable + pending
}
func recoveryDisplayCounts(m Model) [4]uint64 {
	total := m.Recovery.TotalSectors
	if total == 0 {
		return [4]uint64{0, 0, 0, 1}
	}
	remaining := total
	recovered := recoveryMinUint64(m.Recovery.RecoveredSectors, remaining)
	remaining -= recovered
	deferred := recoveryMinUint64(m.Recovery.DeferredSectors, remaining)
	remaining -= deferred
	unreadable := recoveryMinUint64(m.Recovery.UnreadableSectors, remaining)
	remaining -= unreadable
	covered := recoveryCoveredSectors(m)
	known := recovered + deferred + unreadable
	if covered > known {
		recovered += recoveryMinUint64(covered-known, remaining)
		remaining = total - recovered - deferred - unreadable
	}
	return [4]uint64{recovered, deferred, unreadable, remaining}
}
func recoveryMinUint64(left, right uint64) uint64 {
	if left < right {
		return left
	}
	return right
}
func allocateRecoveryBarCells(width int, counts [4]uint64, total uint64) [4]int {
	if total == 0 {
		return [4]int{0, 0, 0, width}
	}
	var cells [4]int
	var assigned int
	for i, count := range counts {
		cells[i] = int((count * uint64(width)) / total)
		assigned += cells[i]
	}
	for i, count := range counts {
		if count > 0 && cells[i] == 0 && assigned < width {
			cells[i] = 1
			assigned++
		}
	}
	for assigned < width {
		for i, count := range counts {
			if assigned == width {
				break
			}
			if count > 0 {
				cells[i]++
				assigned++
			}
		}
	}
	for assigned > width {
		for i := len(cells) - 1; i >= 0 && assigned > width; i-- {
			if cells[i] > 0 {
				cells[i]--
				assigned--
			}
		}
	}
	return cells
}
func recoveryCoveredSectors(m Model) uint64 {
	if m.Recovery.TotalSectors == 0 {
		return 0
	}
	covered := m.Recovery.RecoveredSectors + m.Recovery.DeferredSectors + m.Recovery.UnreadableSectors
	if m.Recovery.ScannedSectors > covered {
		covered = m.Recovery.ScannedSectors
	}
	if covered > m.Recovery.TotalSectors {
		return m.Recovery.TotalSectors
	}
	return covered
}
func renderPausingPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	action := "Pause requested"
	detail := "Pause requested. Waiting for the current drive request to finish safely."
	checkpoint := "No new drive commands will be started until the recovery is fully paused."
	if m.Recovery.StopPending {
		action = "Saving progress and stopping"
		detail = "Stop requested. Waiting for the current drive request to finish safely."
		checkpoint = "The image and recovery map will remain resumable after the checkpoint is saved."
	}
	lines := []string{
		theme.Warning.Render(m.LoadingSpinner.View() + " " + action),
		"",
	}
	lines = append(lines, wrapText(detail, width-4)...)
	lines = append(lines, wrapText(checkpoint, width-4)...)
	if m.Recovery.ForceStopAvailable {
		lines = append(lines, "", theme.Warning.Render("The current drive request has not returned."))
		lines = append(lines, theme.Muted.Render("Press x to force-stop the active device request; ctrl+c also works."))
	}
	if m.Recovery.OutputPath != "" && m.Recovery.OutputPath != "Not chosen yet" {
		lines = append(lines, "")
		lines = append(lines, labeledLines("Output", m.Recovery.OutputPath, width)...)
	}
	return cardLines(theme, "Saving a safe checkpoint", lines, width, true)
}
func renderPausedPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	options := []string{
		"Continue recovery",
		"Stop after checkpoint",
	}
	lines := cardLines(theme, "Paused safely", []string{
		theme.Success.Render("✓ Recovery is paused at a durable checkpoint."),
		"The current image and recovery map are safe to resume.",
		"No new drive commands will be started while paused.",
	}, width, false)
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Next action", choiceMenu(theme, options, m.Cursor, width-4), width, true)...)
	return lines
}
func renderStopConfirmPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	options := []string{
		"Save progress and stop",
		"Continue recovery",
	}
	lines := []string{theme.Warning.Render("△ Stop after a durable checkpoint?"), "", "The image can be resumed later.", "Saving progress is the recommended choice.", ""}
	lines = append(lines, choiceMenu(theme, options, m.Cursor, width-4)...)
	return cardLines(theme, "Confirm stop", lines, width, true)
}
func renderEjectConfirmPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	options := []string{"Force eject", "Cancel"}
	drive := firstNonEmpty(m.SelectedDrive.DisplayName, m.SelectedDrive.Path, "selected drive")
	innerWidth := maxInt(16, width-8)
	guidance := []string{
		"Use force eject only if normal eject did not work for " + drive + ".",
		"Close apps using the drive before continuing.",
		"Recovery must already be stopped; unsaved device work may be abandoned.",
		"DiscRescue will refresh the drive state after the native request.",
	}
	lines := append([]string{}, calloutLines(theme, "Risk", "Force eject may interrupt a drive still in use.", innerWidth)...)
	lines = append(lines, "")
	if innerWidth >= 72 {
		guidanceWidth := (innerWidth * 2) / 3
		actionWidth := innerWidth - guidanceWidth - 1
		guidancePanel := strings.Join(cardLines(theme, "Before continuing", bulletLines(guidance, guidanceWidth-6), guidanceWidth, false), "\n")
		actionPanel := strings.Join(cardLines(theme, "Choose action", choiceMenu(theme, options, m.Cursor, actionWidth-4), actionWidth, true), "\n")
		lines = append(lines, strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, guidancePanel, " ", actionPanel), "\n")...)
	} else {
		lines = append(lines, cardLines(theme, "Before continuing", bulletLines(guidance, innerWidth-6), innerWidth, false)...)
		lines = append(lines, "")
		lines = append(lines, cardLines(theme, "Choose action", choiceMenu(theme, options, m.Cursor, innerWidth-4), innerWidth, true)...)
	}
	return cardLines(theme, "Confirm force eject", lines, width, true)
}
func calloutLines(theme Theme, title, message string, width int) []string {
	contentWidth := maxInt(12, width-4)
	content := []string{theme.Warning.Render(strings.ToUpper(title))}
	for _, line := range wrapText("△ "+message, maxInt(10, contentWidth-2)) {
		content = append(content, theme.Warning.Render(line))
	}
	return strings.Split(theme.NoticeWarning.Width(contentWidth).Render(strings.Join(content, "\n")), "\n")
}
func bulletLines(items []string, width int) []string {
	lines := make([]string, 0, len(items))
	for _, item := range items {
		for _, line := range wrapText("• "+item, maxInt(12, width)) {
			lines = append(lines, line)
		}
	}
	return lines
}

func renderCompactRecoveryPage(m Model, width int) ([]string, bool) {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	switch m.Page {
	case PagePausing:
		return []string{
			theme.Warning.Render(m.LoadingSpinner.View() + " Pause requested"),
			"Finishing the current drive request safely.",
			"No new drive command will be started.",
		}, true
	case PagePaused:
		lines := []string{theme.Success.Render("\u2713 Paused at a durable checkpoint.")}
		return append(lines, choiceMenu(theme, []string{"Continue recovery", "Stop after checkpoint"}, m.Cursor, width)...), true
	case PageStopConfirm:
		lines := []string{theme.Warning.Render("\u25b3 Save progress before stopping?"), "The recovery can be resumed later."}
		return append(lines, choiceMenu(theme, []string{"Save progress and stop", "Continue recovery"}, m.Cursor, width)...), true
	default:
		return nil, false
	}
}
