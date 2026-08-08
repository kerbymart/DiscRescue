package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"discrescue/internal/buildinfo"
)

type layoutTier uint8

const (
	layoutFull layoutTier = iota
	layoutMedium
	layoutCompact
	layoutTooSmall
)

const (
	progressEmptyCell = "\u2591"
	progressFullCell  = "\u2588"
)

var progressPartialCells = []string{
	"\u258F",
	"\u258E",
	"\u258D",
	"\u258C",
	"\u258B",
	"\u258A",
	"\u2589",
}

func (m Model) View() tea.View {
	tier := layoutTierFor(m.Width, m.Height)
	if tier == layoutTooSmall {
		lines := []string{
			"DiscRescue",
			"",
			"Window too small.",
			"Resize to at least 40x12.",
		}
		if m.Page == PageRecovering || m.Page == PagePausing || m.Page == PagePaused || m.Page == PageStopConfirm || m.Page == PageDetails {
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
	return renderShell(m, renderPageBody(m, width, tier), tier)
}

func usesAltScreen(page Page) bool {
	return page == PageRecovering || page == PagePausing || page == PageDetails
}

func pageTitle(m Model) string {
	switch m.Page {
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
		if name := recoveryOutputName(m); name != "" {
			return "Recovering " + name
		}
		return "Recovery in progress"
	case PagePausing:
		if name := recoveryOutputName(m); name != "" {
			return "Pausing " + name
		}
		return "Pausing recovery"
	case PagePaused:
		if name := recoveryOutputName(m); name != "" {
			return "Recovery paused — " + name
		}
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
	if body, ok := renderTightPageBody(m, width, tier); ok {
		for i := range body {
			body[i] = fitToWidth(body[i], width)
		}
		return body
	}
	switch m.Page {
	case PageDiscover:
		theme := newTheme(m.Monochrome, m.DarkBackground)
		return cardLines(theme, "Drive discovery", []string{
			theme.Accent.Render(m.LoadingSpinner.View() + " Looking for optical drives"),
			"",
			theme.Muted.Render("Checking the devices available to this computer."),
			theme.Muted.Render("This usually takes only a moment."),
		}, width, true)
	case PageNoDrives:
		return renderNoDrivesPage(m, width)
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
	case PagePausing:
		return renderPausingPage(m, width)
	case PagePaused:
		return renderPausedPage(m, width)
	case PageStopConfirm:
		return renderStopConfirmPage(m, width)
	case PageSummary:
		return renderSummaryPage(m, width, tier)
	case PageResumeJobs:
		return renderResumeJobsPage(m, width, tier)
	case PageHistory:
		return renderHistoryPage(m, width, tier)
	case PageDetails:
		return renderDetailsPage(m, width)
	case PageAdvanced:
		theme := newTheme(m.Monochrome, m.DarkBackground)
		return cardLines(theme, "Advanced recovery", []string{"Advanced settings stay separate from the normal setup flow.", "", theme.Muted.Render("The guided defaults protect resumability and media safety.")}, width, false)
	case PageAbout:
		return renderAboutPage(m, width)
	default:
		return nil
	}
}

// renderTightPageBody keeps the primary state and action visible when the
// terminal has only six to nine body rows. Component-rich layouts resume as
// soon as there is enough vertical space for their borders and descriptions.
func renderTightPageBody(m Model, width int, tier layoutTier) ([]string, bool) {
	if tier != layoutCompact && m.Height >= 20 {
		return nil, false
	}
	theme := newTheme(m.Monochrome, m.DarkBackground)
	switch m.Page {
	case PageDiscover:
		return []string{theme.Accent.Render(m.LoadingSpinner.View() + " Looking for optical drives"), "Checking the devices available to this computer."}, true
	case PageNoDrives:
		return []string{theme.Warning.Render("△ No readable optical drive is available."), "Insert a CD or DVD, then press Enter to retry."}, true
	case PageDiscoveryError:
		message := "Drive discovery failed."
		if m.Notice != nil && m.Notice.Text != "" {
			message = m.Notice.Text
		}
		return []string{fitToWidth(theme.Danger.Render("× "+message), width), "> Retry discovery"}, true
	case PageInspectingMedia:
		return []string{
			theme.Accent.Render(m.LoadingSpinner.View() + " Reading disc information"),
			fitToWidth("Drive  "+selectedDriveLabel(m), width),
			"Checking capacity, layout, and matching work.",
		}, true
	case PagePriorProcessing:
		lines := []string{fitToWidth(theme.Accent.Render(firstNonEmpty(m.PriorView.Title, "Matching contents")), width)}
		if len(m.PriorView.Body) > 0 {
			lines = append(lines, fitToWidth(m.PriorView.Body[0], width))
		}
		if len(m.PriorView.Options) > 0 {
			lines = append(lines, choiceMenu(theme, []string{m.PriorView.Options[clampIndex(m.Cursor, len(m.PriorView.Options))]}, 0, width)...)
			lines = append(lines, fmt.Sprintf("↑/↓ choose from %d actions", len(m.PriorView.Options)))
		}
		return lines, true
	case PageChooseAction:
		return choiceMenu(theme, []string{"Start a new recovery", "Resume an unfinished recovery", "Browse processed media", "Choose another drive"}, m.Cursor, width), true
	case PageReview:
		lines := []string{fitToWidth("Drive   "+selectedDriveLabel(m), width), fitToWidth("Output  "+m.Setup.OutputPath, width)}
		return append(lines, choiceMenu(theme, []string{firstNonEmpty(m.Setup.ActionLabel, "Start a new recovery"), "Edit output path", "Choose another drive"}, m.Cursor, width)...), true
	case PagePausing:
		return []string{theme.Warning.Render(m.LoadingSpinner.View() + " Pause requested"), "Finishing the current drive request safely.", "No new drive command will be started."}, true
	case PagePaused:
		lines := []string{theme.Success.Render("✓ Paused at a durable checkpoint.")}
		return append(lines, choiceMenu(theme, []string{"Continue recovery", "Stop after checkpoint"}, m.Cursor, width)...), true
	case PageStopConfirm:
		lines := []string{theme.Warning.Render("△ Save progress before stopping?"), "The recovery can be resumed later."}
		return append(lines, choiceMenu(theme, []string{"Save progress and stop", "Continue recovery"}, m.Cursor, width)...), true
	case PageAdvanced:
		return []string{"Advanced recovery settings stay separate.", "Guided defaults protect resumability and media safety."}, true
	case PageAbout:
		return []string{theme.Accent.Render("◉ DiscRescue"), "Guided optical-disc recovery.", "Version " + buildinfo.Version, "Commit " + buildinfo.Commit}, true
	default:
		return nil, false
	}
}

func clampIndex(index, length int) int {
	if length <= 0 || index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}

func cardLines(theme Theme, title string, content []string, width int, focused bool) []string {
	inner := maxInt(12, width-4)
	style := theme.Card
	if focused {
		style = theme.CardFocus
	}
	body := strings.Join(content, "\n")
	if title != "" {
		body = theme.Label.Render(strings.ToUpper(title)) + "\n" + body
	}
	return strings.Split(style.Width(inner).Render(body), "\n")
}

func choiceMenu(theme Theme, options []string, selected, width int) []string {
	lines := make([]string, 0, len(options))
	for i, option := range options {
		marker := "  "
		style := theme.Text.Padding(0, 1)
		if i == selected {
			marker = "> "
			style = theme.Focus.Padding(0, 1).Width(maxInt(8, width-2))
		}
		lines = append(lines, style.Render(fitToWidth(marker+option, maxInt(8, width-4))))
	}
	return lines
}

func metricStrip(theme Theme, metrics [][2]string, width int) []string {
	if len(metrics) == 0 {
		return nil
	}
	gap := len(metrics) - 1
	cellWidth := (width - gap) / len(metrics)
	if cellWidth < 14 {
		lines := make([]string, 0, len(metrics))
		for _, metric := range metrics {
			lines = append(lines, theme.Label.Render(metric[0])+"  "+theme.Text.Render(metric[1]))
		}
		return lines
	}
	cards := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		cards = append(cards, theme.Card.Width(cellWidth).Render(theme.Label.Render(strings.ToUpper(metric[0]))+"\n"+theme.Text.Render(metric[1])))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, intersperse(cards, " ")...), "\n")
}

func intersperse(values []string, separator string) []string {
	if len(values) < 2 {
		return values
	}
	out := make([]string, 0, len(values)*2-1)
	for i, value := range values {
		if i > 0 {
			out = append(out, separator)
		}
		out = append(out, value)
	}
	return out
}

func renderDeviceList(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.DriveList.Width() > 0 && len(m.DriveList.Items()) > 0 {
		return cardLines(theme, "Available drives", strings.Split(m.DriveList.View(), "\n"), width, true)
	}
	if len(m.Devices) == 0 {
		return cardLines(theme, "Available drives", []string{"No usable optical drives found."}, width, true)
	}
	lines := make([]string, 0, len(m.Devices)*2)
	for i, device := range m.Devices {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, prefix+device.DisplayName, "  "+device.Path+" · "+device.Status)
	}
	return cardLines(theme, "Available drives", lines, width, true)
}

func renderNoDrivesPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	return cardLines(theme, "No drives found", []string{
		theme.Warning.Render("△ No readable optical drive is available."),
		"",
		"Insert a CD or DVD, then retry discovery.",
		theme.Muted.Render("Press Enter to retry or q to quit."),
	}, width, false)
}

func renderDiscoveryErrorPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	text := "Drive discovery failed."
	if m.Notice != nil && m.Notice.Text != "" {
		text = m.Notice.Text
	}
	return cardLines(theme, "Discovery needs attention", []string{
		theme.Danger.Render("× " + text),
		"",
		"Press Enter to retry discovery or q to quit.",
	}, width, false)
}

func renderInspectingMediaPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	lines := []string{theme.Accent.Render(m.LoadingSpinner.View() + " Reading the disc layout and identity")}
	if m.SelectedDrive.DisplayName != "" {
		lines = append(lines, "")
		lines = append(lines, theme.Label.Render("DRIVE")+"  "+m.SelectedDrive.DisplayName)
	}
	if m.SelectedDrive.Path != "" {
		lines = append(lines, theme.Label.Render("PATH")+"   "+m.SelectedDrive.Path)
	}
	lines = append(lines, "")
	lines = append(lines, theme.Muted.Render("DiscRescue is checking capacity, layout, and matching local work."))
	return cardLines(theme, "Media inspection", lines, width, true)
}

func renderPriorProcessing(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.PriorView.Kind == PriorProcessingNone || m.PriorView.Kind == PriorProcessingIndeterminate {
		return cardLines(theme, "Matching contents", wrapText(m.PriorView.HistoryLine, width-4), width, false)
	}

	result := append([]string{theme.Accent.Render(m.PriorView.Title)}, wrapText(strings.Join(m.PriorView.Body, " "), width-4)...)
	if m.PriorView.ImagePath != "" {
		result = append(result, "", theme.Label.Render("IMAGE")+"  "+m.PriorView.ImagePath)
	}
	if tier != layoutCompact && m.PriorView.CopyLabel != "" {
		result = append(result, theme.Label.Render("COPY")+"   "+m.PriorView.CopyLabel)
	}
	if tier == layoutFull && m.PriorView.LastSaved != "" {
		result = append(result, theme.Label.Render("SAVED")+"  "+m.PriorView.LastSaved)
	}
	if tier == layoutFull && m.PriorView.Recovered != "" {
		result = append(result, theme.Success.Render("RECOVERED")+"  "+m.PriorView.Recovered)
	}
	if tier == layoutFull && m.PriorView.UnreadableSectors != "" {
		result = append(result, theme.Danger.Render("UNREADABLE")+"  "+m.PriorView.UnreadableSectors)
	}
	lines := cardLines(theme, "Matching contents", result, width, false)
	if len(m.PriorView.Options) > 0 {
		lines = append(lines, "")
		lines = append(lines, cardLines(theme, "Next action", choiceMenu(theme, m.PriorView.Options, m.Cursor, width-4), width, true)...)
	}
	return lines
}

func renderActionList(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	lines := []string{}
	if m.ActionList.Width() > 0 && m.Height >= 24 {
		lines = append(lines, cardLines(theme, "Recovery actions", strings.Split(m.ActionList.View(), "\n"), width, true)...)
	} else {
		actions := []string{"Start a new recovery", "Resume an unfinished recovery", "Browse processed media", "Choose another drive"}
		lines = append(lines, cardLines(theme, "Recovery actions", choiceMenu(theme, actions, m.Cursor, width-4), width, true)...)
	}
	if tier != layoutCompact {
		lines = append(lines, "")
		context := []string{theme.Muted.Render(firstNonEmpty(m.PriorView.HistoryLine, "Checking this computer for matching saved work."))}
		if m.Identity.Detail != "" {
			context = append(context, "Disc: "+m.Identity.Detail)
		}
		if m.Height >= 30 {
			lines = append(lines, cardLines(theme, "Current media", context, width, false)...)
		} else {
			lines = append(lines, context...)
		}
	}
	return lines
}

func renderResumeJobsPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.ResumeList.Width() > 0 && len(m.ResumeList.Items()) > 0 {
		return cardLines(theme, "Saved recoveries", strings.Split(m.ResumeList.View(), "\n"), width, true)
	}
	if len(m.ResumeJobs) == 0 {
		lines := wrapText("No resumable recoveries were found in the current output folder.", width)
		lines = append(lines, "")
		lines = append(lines, wrapText("Press Enter or Esc to go back.", width)...)
		return lines
	}

	lines := wrapText("Select a saved recovery that matches the current disc contents.", width)
	for i, job := range m.ResumeJobs {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, "")
		lines = append(lines, wrapText(prefix+job.OutputPath, width)...)
		if tier != layoutCompact {
			lines = append(lines, labeledLines("Resume", job.Detail, width)...)
		}
	}
	lines = append(lines, "")
	backPrefix := "  "
	if m.Cursor >= len(m.ResumeJobs) {
		backPrefix = "> "
	}
	lines = append(lines, wrapText(backPrefix+"Back", width)...)
	return lines
}

func renderHistoryPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.HistoryList.Width() > 0 && len(m.HistoryList.Items()) > 0 {
		return cardLines(theme, "Processed media", strings.Split(m.HistoryList.View(), "\n"), width, true)
	}
	if len(m.HistoryItems) == 0 {
		lines := wrapText("No processed media were found in the current output folder.", width)
		lines = append(lines, "")
		lines = append(lines, wrapText("Press Enter or Esc to go back.", width)...)
		return lines
	}

	lines := wrapText("Browse saved recovery images and their local recovery maps.", width)
	for i, item := range m.HistoryItems {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, "")
		lines = append(lines, wrapText(prefix+item.Title, width)...)
		if tier != layoutCompact {
			lines = append(lines, labeledLines("Status", item.Status, width)...)
			if item.ModifiedAt != "" {
				lines = append(lines, labeledLines("Updated", item.ModifiedAt, width)...)
			}
		}
	}
	lines = append(lines, "")
	backPrefix := "  "
	if m.Cursor >= len(m.HistoryItems) {
		backPrefix = "> "
	}
	lines = append(lines, wrapText(backPrefix+"Back", width)...)
	return lines
}

func renderOutputPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	directory := m.Setup.OutputDirectory
	if directory == "" {
		directory = " "
	}
	fileName := m.Setup.OutputFileName
	if fileName == "" {
		fileName = " "
	}
	if tier == layoutCompact || m.Height < 28 {
		if m.Setup.OutputEditing && m.Setup.ActiveOutputField == OutputFieldDirectory {
			directory = m.DirectoryInput.View()
		}
		if m.Setup.OutputEditing && m.Setup.ActiveOutputField == OutputFieldFileName {
			fileName = m.FileNameInput.View()
		}
		lines := []string{
			compactFieldLine(theme, "Folder", directory, width, m.Cursor == 0),
			compactFieldLine(theme, "File name", fileName, width, m.Cursor == 1),
			fitToWidth("Full path  "+firstNonEmpty(m.Setup.OutputPath, "Not chosen yet"), width),
		}
		if tier != layoutCompact {
			lines = append(lines, fitToWidth("Space      "+m.Setup.FreeSpace, width))
		}
		lines = append(lines, choiceMenu(theme, []string{"Continue with this target"}, boolIndex(m.Cursor == 2), width)...)
		return lines
	}
	lines := []string{theme.Muted.Render("Use Enter to edit a field. Tab moves focus while editing.")}
	lines = append(lines, "")
	if m.Setup.OutputEditing && m.Setup.ActiveOutputField == OutputFieldDirectory {
		lines = append(lines, outputFieldLines(theme, "Folder", m.DirectoryInput.View(), width, true, true)...)
	} else {
		lines = append(lines, outputFieldLines(theme, "Folder", directory, width, m.Cursor == 0, false)...)
	}
	lines = append(lines, "")
	if m.Setup.OutputEditing && m.Setup.ActiveOutputField == OutputFieldFileName {
		lines = append(lines, outputFieldLines(theme, "File name", m.FileNameInput.View(), width, true, true)...)
	} else {
		lines = append(lines, outputFieldLines(theme, "File name", fileName, width, m.Cursor == 1, false)...)
	}
	metadata := []string{
		theme.Label.Render("FULL PATH") + "  " + firstNonEmpty(m.Setup.OutputPath, "Not chosen yet"),
		theme.Label.Render("FORMAT") + "     " + m.Setup.OutputFormat,
	}
	if m.Setup.DefaultPath != "" && m.Setup.DefaultPath != "Not chosen yet" {
		metadata = append(metadata, theme.Label.Render("SUGGESTED")+"  "+m.Setup.DefaultPath)
	}
	if m.Setup.ResumeReady {
		metadata = append(metadata, theme.Success.Render("RESUME")+"     "+firstNonEmpty(m.Setup.ResumeDetail, "This target can resume a previous recovery."))
	}
	if tier != layoutCompact {
		metadata = append(metadata, theme.Label.Render("SPACE")+"      "+m.Setup.FreeSpace)
	}
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Target", metadata, width, false)...)
	lines = append(lines, "")
	lines = append(lines, choiceMenu(theme, []string{"Continue with this target"}, boolIndex(m.Cursor == 2), width)...)
	return lines
}

func compactFieldLine(theme Theme, label, value string, width int, selected bool) string {
	line := fmt.Sprintf("%-10s %s", label, value)
	if selected {
		return theme.Focus.Width(width).Render("> " + fitToWidth(line, maxInt(8, width-2)))
	}
	return theme.Text.Render("  " + fitToWidth(line, maxInt(8, width-2)))
}

func boolIndex(selected bool) int {
	if selected {
		return 0
	}
	return -1
}

func outputFieldLines(theme Theme, label, value string, width int, selected, editing bool) []string {
	caption := theme.Label.Render(strings.ToUpper(label))
	if editing {
		caption += "  " + theme.Badge.Render("EDITING")
	}
	style := theme.Card
	if selected {
		style = theme.CardFocus
	}
	inner := maxInt(12, width-4)
	field := style.Width(inner).Render(value)
	return append([]string{caption}, strings.Split(field, "\n")...)
}

func renderReviewPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	options := []string{
		firstNonEmpty(m.Setup.ActionLabel, "Start a new recovery"),
		"Edit output path",
		"Choose another drive",
	}
	if tier == layoutCompact || m.Height < 28 {
		lines := []string{
			"Drive   " + selectedDriveLabel(m),
			"Disc    " + discSummary(m),
			"Output  " + m.Setup.OutputPath,
			"Mode    " + firstNonEmpty(m.Setup.MethodLabel, "Balanced recovery"),
		}
		if m.Setup.ResumeReady {
			lines = append(lines, "Map     "+m.Setup.ResumeMapPath)
			if m.Setup.ResumeDetail != "" && tier != layoutCompact {
				lines = append(lines, wrapText(m.Setup.ResumeDetail, width)...)
			}
		}
		lines = append(lines, "")
		lines = append(lines, choiceMenu(theme, options, m.Cursor, width)...)
		return lines
	}
	context := []string{theme.Label.Render("DRIVE") + "  " + selectedDriveLabel(m), theme.Label.Render("DISC") + "   " + discSummary(m)}
	target := []string{theme.Label.Render("IMAGE") + "  " + m.Setup.OutputPath, theme.Label.Render("MODE") + "   " + firstNonEmpty(m.Setup.MethodLabel, "Balanced recovery")}
	if m.Setup.ResumeReady {
		target = append(target, theme.Success.Render("MAP")+"    "+m.Setup.ResumeMapPath)
	}
	lines := cardLines(theme, "Disc", context, width, false)
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Output", target, width, false)...)
	if m.Setup.ResumeDetail != "" {
		lines = append(lines, "")
		lines = append(lines, theme.Muted.Render(m.Setup.ResumeDetail))
	}
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Ready", choiceMenu(theme, options, m.Cursor, width-4), width, true)...)
	return lines
}

func renderRecoveryPage(m Model, width int, tier layoutTier) []string {
	return renderRecoveryDashboard(m, width, tier)
}

func renderRecoveryDashboard(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	phase := firstNonEmpty(m.Recovery.Phase, "Preparing recovery")
	status := firstNonEmpty(m.Recovery.Status, "Reading sectors from the selected optical drive.")
	if tier == layoutCompact || m.Height < 28 {
		if tier == layoutCompact {
			lines := []string{theme.Accent.Render(phase), recoveryProgressLine(m, width, tier)}
			lines = append(lines,
				fmt.Sprintf("Recovered  %s sectors", formatCount(m.Recovery.RecoveredSectors)),
				fitToWidth(fmt.Sprintf("Deferred %s  •  Unreadable %s", formatCount(m.Recovery.DeferredSectors), formatCount(m.Recovery.UnreadableSectors)), width),
			)
			if summary := recoveryTimeSummary(m, tier); summary != "" {
				lines = append(lines, summary)
			}
			return lines
		}
		lines := []string{theme.Accent.Render(phase), fitToWidth(theme.Muted.Render(status), width), recoveryProgressLine(m, width, tier)}
		if m.Recovery.TotalSectors > 0 {
			lines = append(lines, fmt.Sprintf("Scanned      %s of %s sectors", formatCount(m.Recovery.ScannedSectors), formatCount(m.Recovery.TotalSectors)))
		}
		lines = append(lines,
			fmt.Sprintf("Recovered    %s sectors", formatCount(m.Recovery.RecoveredSectors)),
			fmt.Sprintf("Deferred     %s sectors", formatCount(m.Recovery.DeferredSectors)),
			fmt.Sprintf("Unreadable   %s sectors", formatCount(m.Recovery.UnreadableSectors)),
		)
		if summary := recoveryTimeSummary(m, tier); summary != "" {
			lines = append(lines, summary)
		}
		if tier != layoutCompact && (m.Recovery.Throughput != "" || m.Recovery.Elapsed != "") {
			lines = append(lines, "Rate  "+firstNonEmpty(m.Recovery.Throughput, "—")+"  •  Elapsed  "+firstNonEmpty(m.Recovery.Elapsed, "—"))
		}
		if tier != layoutCompact && m.Height >= 24 && len(m.Recovery.LastIssue) > 0 {
			lines = append(lines, "Last issue  "+m.Recovery.LastIssue[0])
		}
		return lines
	}

	lines := []string{theme.Badge.Render(phase), theme.Muted.Render(status), "", recoveryProgressLine(m, width, tier)}
	if m.Recovery.TotalSectors > 0 {
		lines = append(lines, theme.Muted.Render(fmt.Sprintf("%s / %s sectors covered", formatCount(m.Recovery.ScannedSectors), formatCount(m.Recovery.TotalSectors))))
	}
	lines = append(lines, "")
	metrics := [][2]string{
		{"Recovered", formatCount(m.Recovery.RecoveredSectors) + " sectors"},
		{"Deferred", formatCount(m.Recovery.DeferredSectors) + " sectors"},
		{"Unreadable", formatCount(m.Recovery.UnreadableSectors) + " sectors"},
	}
	lines = append(lines, metricStrip(theme, metrics, width)...)
	timing := make([]string, 0, 3)
	if summary := recoveryTimeSummary(m, tier); summary != "" {
		timing = append(timing, summary)
	}
	if m.Recovery.Throughput != "" || m.Recovery.Elapsed != "" {
		timing = append(timing, "Rate  "+firstNonEmpty(m.Recovery.Throughput, "—")+"    Elapsed  "+firstNonEmpty(m.Recovery.Elapsed, "—"))
	}
	if len(timing) > 0 {
		lines = append(lines, "")
		lines = append(lines, cardLines(theme, "Timing", timing, width, false)...)
	}
	if len(m.Recovery.LastIssue) > 0 {
		issue := make([]string, 0, len(m.Recovery.LastIssue))
		for _, line := range m.Recovery.LastIssue {
			issue = append(issue, wrapText(line, width-4)...)
		}
		lines = append(lines, "")
		lines = append(lines, cardLines(theme, "Last issue", issue, width, false)...)
	}
	return lines
}

func recoveryProgressView(m Model, width int) string {
	barWidth := width - 6
	if barWidth < 8 {
		barWidth = 8
	}
	p := progress.New(
		progress.WithoutPercentage(),
		progress.WithFillCharacters('━', '┈'),
	)
	if !m.Monochrome {
		p = progress.New(
			progress.WithoutPercentage(),
			progress.WithFillCharacters('━', '┈'),
			progress.WithColors(lipgloss.Color("#6155F5"), lipgloss.Color("#FF4FD8")),
		)
	}
	p.SetWidth(barWidth)
	percent := 0.0
	if m.Recovery.TotalSectors > 0 {
		percent = float64(m.Recovery.ScannedSectors) / float64(m.Recovery.TotalSectors)
		if percent > 1 {
			percent = 1
		}
	}
	return p.ViewAs(percent)
}

func recoveryProgressLine(m Model, width int, tier layoutTier) string {
	// Older snapshots can contain only cumulative recovery counts. Keep their
	// established bar until pass coverage is known; active recovery snapshots
	// always provide ScannedSectors and use the new thin gradient rail.
	if tier == layoutCompact || m.Monochrome || (m.Recovery.ScannedSectors == 0 && m.Recovery.RecoveredSectors > 0) {
		bar := progressBarFor(m, tier)
		percent := 0
		if m.Recovery.TotalSectors > 0 {
			covered := m.Recovery.ScannedSectors
			if covered == 0 {
				covered = m.Recovery.RecoveredSectors
			}
			percent = int((covered * 100) / m.Recovery.TotalSectors)
		}
		return fitToWidth(fmt.Sprintf("%s %d%%", bar, percent), width)
	}
	percent := 0
	if m.Recovery.TotalSectors > 0 {
		percent = int((m.Recovery.ScannedSectors * 100) / m.Recovery.TotalSectors)
		if percent > 100 {
			percent = 100
		}
	}
	bar := recoveryProgressView(m, width)
	return lipgloss.JoinHorizontal(lipgloss.Bottom, bar, " ", fmt.Sprintf("%d%%", percent))
}

func renderSummaryPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	statusMarker := "✓"
	statusStyle := theme.Success
	if m.Recovery.UnreadableSectors > 0 {
		statusMarker = "×"
		statusStyle = theme.Danger
	} else if m.Recovery.DeferredSectors > 0 || strings.Contains(strings.ToLower(m.Recovery.Status), "paused") {
		statusMarker = "△"
		statusStyle = theme.Warning
	}
	if tier == layoutCompact {
		lines := []string{
			fitToWidth(statusStyle.Render(statusMarker+" "+m.Recovery.Status), width),
			fitToWidth("Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath), width),
			fmt.Sprintf("Recovered  %s / %s", formatCount(m.Recovery.RecoveredSectors), formatCount(m.Recovery.TotalSectors)),
		}
		lines = append(lines, choiceMenu(theme, summaryOptions(m), m.Cursor, width)...)
		return lines
	}
	if m.Height < 28 {
		lines := []string{fitToWidth(statusStyle.Render(statusMarker+" "+m.Recovery.Status), width)}
		if m.Recovery.UnreadableSectors > 0 {
			lines = append(lines, fitToWidth(fmt.Sprintf("%s sectors could not be recovered.", formatCount(m.Recovery.UnreadableSectors)), width))
		}
		lines = append(lines,
			fitToWidth("Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath), width),
			fitToWidth("Map        "+firstNonEmpty(m.Summary.MapPath, replaceExtension(m.Recovery.OutputPath, ".drmap")), width),
			fmt.Sprintf("Recovered  %s / %s", formatCount(m.Recovery.RecoveredSectors), formatCount(m.Recovery.TotalSectors)),
		)
		if m.Summary.Duration != "" {
			lines = append(lines, fitToWidth("Duration   "+m.Summary.Duration, width))
		}
		lines = append(lines, choiceMenu(theme, summaryOptions(m), m.Cursor, width)...)
		return lines
	}
	result := []string{statusStyle.Render(statusMarker + " " + m.Recovery.Status), ""}
	if m.Recovery.UnreadableSectors == 0 {
		result = append(result, "Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath))
		result = append(result,
			fitToWidth(fmt.Sprintf("Recovered  %s of %s sectors", formatCount(m.Recovery.RecoveredSectors), formatCount(m.Recovery.TotalSectors)), width),
		)
		if m.Recovery.DeferredSectors > 0 {
			result = append(result, fitToWidth(fmt.Sprintf("Deferred   %s sectors remain for a later pass", formatCount(m.Recovery.DeferredSectors)), width))
		}
		if tier != layoutCompact && m.Summary.Duration != "" {
			result = append(result, fitToWidth("Duration   "+m.Summary.Duration, width))
		}
	} else {
		result = append(result,
			fitToWidth(fmt.Sprintf("%s sectors could not be recovered.", formatCount(m.Recovery.UnreadableSectors)), width),
			fitToWidth("The image and map are complete enough to inspect or retry later.", width),
			"",
		)
		result = append(result, "Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath))
		if tier != layoutCompact {
			result = append(result, "Map        "+firstNonEmpty(m.Summary.MapPath, replaceExtension(m.Recovery.OutputPath, ".drmap")))
			result = append(result, "History    "+firstNonEmpty(m.Summary.CatalogStatus, "Recorded in local processed-media catalog"))
		}
	}
	lines := cardLines(theme, "Result", result, width, false)
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Next action", choiceMenu(theme, summaryOptions(m), m.Cursor, width-4), width, true)...)
	return lines
}

func renderDetailsPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.DetailsViewport.Width() > 1 && m.DetailsViewport.Height() > 1 {
		return cardLines(theme, "Recovery details", strings.Split(m.DetailsViewport.View(), "\n"), width, true)
	}
	lines := make([]string, 0, len(detailsLinesForView(m)))
	for _, line := range detailsLinesForView(m) {
		lines = append(lines, wrapText(line, width)...)
	}
	return cardLines(theme, "Recovery details", lines, width, true)
}

func renderPausingPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	lines := []string{
		theme.Warning.Render(m.LoadingSpinner.View() + " Pause requested"),
		"",
	}
	lines = append(lines, wrapText("Pause requested. Waiting for the current drive request to finish safely.", width-4)...)
	lines = append(lines, wrapText("No new drive commands will be started until the recovery is fully paused.", width-4)...)
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

func renderAboutPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	lines := []string{
		theme.Accent.Render("◉ DiscRescue"),
		"A guided optical-disc recovery tool.",
		"",
		"Version " + buildinfo.Version,
		"Commit " + buildinfo.Commit,
		"Build date " + buildinfo.BuildDate,
	}

	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, wrapText(line, width)...)
	}
	return cardLines(theme, "About", out, width, false)
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
		return 74
	}
	value := shellOuterWidth(width) - 6
	if width < 60 {
		value = width - 4
	}
	if value < 24 {
		value = 24
	}
	if value > 104 {
		value = 104
	}
	return value
}

func renderFooter(page Page, width int, tier layoutTier) string {
	var footer string
	switch page {
	case PageDiscover:
		footer = "q quit"
	case PageChooseOutput:
		if tier == layoutCompact {
			footer = "j/k move  enter edit/select"
		} else {
			footer = "j/k move  -  enter edit/select  -  type edit  -  backspace delete  -  tab switch field  -  esc back"
		}
	case PageRecovering:
		if tier == layoutCompact {
			footer = "space pause  d details"
		} else {
			footer = "space pause  -  d details  -  q stop"
		}
	case PagePausing:
		footer = "d details  -  q stop"
	case PagePaused:
		footer = "j/k select  -  enter choose  -  d details"
	case PageStopConfirm:
		footer = "j/k select  -  enter choose  -  esc continue"
	case PageResumeJobs:
		if tier == layoutCompact {
			footer = "enter select  esc back"
		} else {
			footer = "j/k move  -  enter select  -  esc back"
		}
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

func showStatusLine(page Page, tier layoutTier) bool {
	if tier == layoutCompact {
		return false
	}
	return page != PageDiscover
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return []string{""}
	}
	return strings.Split(lipgloss.Wrap(text, width, ""), "\n")
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
	if width <= 0 {
		return text
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	return ansi.Truncate(text, width, "…")
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

	if m.Recovery.TotalSectors == 0 {
		return "[" + strings.Repeat(".", width) + "]"
	}

	if m.Monochrome || tier == layoutCompact {
		filled := int((m.Recovery.RecoveredSectors * uint64(width)) / m.Recovery.TotalSectors)
		if filled > width {
			filled = width
		}
		return "[" + strings.Repeat("=", filled) + strings.Repeat(".", width-filled) + "]"
	}

	return renderUnicodeProgressBar(width, m.Recovery.RecoveredSectors, m.Recovery.TotalSectors)
}

func renderUnicodeProgressBar(width int, recovered, total uint64) string {
	if total == 0 {
		return "[" + strings.Repeat(progressEmptyCell, width) + "]"
	}

	scaled := (float64(recovered) / float64(total)) * float64(width)
	full := int(scaled)
	if full > width {
		full = width
	}

	partial := 0
	if full < width {
		remainder := scaled - float64(full)
		partial = int(remainder * 8)
	}

	var bar strings.Builder
	bar.Grow(width*3 + 2)
	bar.WriteString("[")
	if full > 0 {
		bar.WriteString(strings.Repeat(progressFullCell, full))
	}
	if partial > 0 && full < width {
		bar.WriteString(progressPartialCells[partial-1])
		full++
	}
	if full < width {
		bar.WriteString(strings.Repeat(progressEmptyCell, width-full))
	}
	bar.WriteString("]")
	return bar.String()
}

func unicodeProgressBar(width int, recovered, total uint64) string {
	if total == 0 {
		return "[" + strings.Repeat("░", width) + "]"
	}
	levels := []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	scaled := (float64(recovered) / float64(total)) * float64(width)
	full := int(scaled)
	if full > width {
		full = width
	}
	partial := 0
	if full < width {
		remainder := scaled - float64(full)
		partial = int(remainder * 8)
	}

	var bar strings.Builder
	bar.Grow(width*3 + 2)
	bar.WriteString("[")
	if full > 0 {
		bar.WriteString(strings.Repeat("█", full))
	}
	if partial > 0 && full < width {
		bar.WriteString(levels[partial-1])
		full++
	}
	if full < width {
		bar.WriteString(strings.Repeat("░", width-full))
	}
	bar.WriteString("]")
	return bar.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

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
			return remaining + " • Estimating time remaining..."
		}
		return remaining
	}
	return remaining + " • " + eta
}

func detailsLinesForView(m Model) []string {
	switch m.PreviousPage {
	case PageHistory:
		if m.Page == PageDetails {
			return append([]string(nil), m.Details.Lines...)
		}
	}

	switch m.Page {
	case PageRecovering, PagePausing, PagePaused:
		return buildRecoveryDetails(m)
	case PageSummary:
		return buildSummaryDetails(m, JobStoppedMsg{Summary: m.Summary, Err: m.LastError})
	case PageDetails:
		switch m.PreviousPage {
		case PageRecovering, PagePausing, PagePaused:
			return buildRecoveryDetails(m)
		case PageSummary:
			return buildSummaryDetails(m, JobStoppedMsg{Summary: m.Summary, Err: m.LastError})
		default:
			return append([]string(nil), m.Details.Lines...)
		}
	default:
		return append([]string(nil), m.Details.Lines...)
	}
}
