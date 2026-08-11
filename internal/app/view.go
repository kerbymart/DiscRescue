package app

import (
	"fmt"
	"strings"

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

func usesAltScreen(_ Page) bool {
	// DiscRescue owns one interactive terminal session. Keep every page in the
	// alternate screen so discovery and setup do not leak into shell history.
	return true
}

func pageTitle(m Model) string {
	switch m.Page {
	case PageDiscover:
		return "Finding usable optical drives"
	case PageNoDrives:
		return "Drive discovery"
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
	case PageEjectConfirm:
		return "Confirm force eject"
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
	case PageEjectConfirm:
		return renderEjectConfirmPage(m, width)
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
	if m.Page == PageEjectConfirm && m.Height < 28 {
		theme := newTheme(m.Monochrome, m.DarkBackground)
		lines := []string{
			theme.Warning.Render("△ Force eject may interrupt a drive in use."),
			"Use only if normal eject did not work.",
		}
		return append(lines, choiceMenu(theme, []string{"Force eject", "Cancel"}, m.Cursor, width)...), true
	}
	if tier != layoutCompact && m.Height >= 20 {
		return nil, false
	}
	theme := newTheme(m.Monochrome, m.DarkBackground)
	switch m.Page {
	case PageDiscover:
		return []string{theme.Accent.Render(m.LoadingSpinner.View() + " Looking for optical drives"), "Checking the devices available to this computer."}, true
	case PageNoDrives:
		return []string{"No optical drive available.", "Connect or insert a readable CD/DVD.", "Press Enter to retry discovery."}, true
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
	case PageDetails:
		preview := "No technical detail is available."
		if content := strings.TrimSpace(m.DetailsViewport.View()); content != "" {
			preview = strings.Split(content, "\n")[0]
		}
		return []string{
			theme.Accent.Render("Recovery details"),
			fitToWidth(preview, width),
			theme.Muted.Render("Use up/down to scroll; esc to return."),
		}, true
	case PageChooseOutput:
		if m.noticeHasTechnicalDetail() {
			return []string{fitToWidth(theme.Danger.Render("× "+m.Notice.String()), width)}, true
		}
		return nil, false
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
		if m.noticeHasTechnicalDetail() {
			return []string{fitToWidth(theme.Danger.Render("× "+m.Notice.String()), width)}, true
		}
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
	return cardLines(theme, "Next step", []string{
		"Connect an optical drive or insert a readable CD/DVD.",
		theme.Muted.Render("Press Enter to retry discovery."),
	}, width, false)
}

func renderDiscoveryErrorPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	return cardLines(theme, "Discovery needs attention", []string{
		"Press Enter to retry discovery.",
		theme.Muted.Render("Press q to quit."),
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
		"Change recovery method",
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
			if history := m.Summary.CatalogStatus.SummaryLine(); history != "" {
				result = append(result, "History    "+history)
			}
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

func renderFooter(m Model, width int, tier layoutTier) string {
	if m.Page == PageRecovering {
		theme := newTheme(m.Monochrome, m.DarkBackground)
		footer := theme.Key.Render("space") + " " + theme.Muted.Render("pause") + "  •  " +
			theme.Key.Render("d") + " " + theme.Muted.Render("details") + "  •  " +
			theme.Key.Render("q") + " " + theme.Muted.Render("stop")
		return fitToWidth(footer, width)
	}
	helpView := FooterHelp(tier == layoutFull)
	helpView.SetWidth(width)
	return fitToWidth(helpView.View(pageHelpForModel(m)), width)
}

func showStatusLine(page Page, tier layoutTier) bool {
	if tier == layoutCompact {
		return false
	}
	return page != PageDiscover && page != PageDetails
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
	options := make([]string, 0, 3)
	if retry := summaryRetryActionLabel(m); retry != "" {
		options = append(options, retry)
	}
	return append(options,
		"Exit",
		"Choose another drive",
	)
}

func summaryRetryActionLabel(m Model) string {
	if m.Recovery.DeferredSectors > 0 && m.Recovery.UnreadableSectors > 0 {
		return "Retry unresolved sectors"
	}
	if m.Recovery.DeferredSectors > 0 {
		return "Retry deferred sectors"
	}
	if m.Recovery.UnreadableSectors > 0 {
		return "Retry unreadable sectors"
	}
	return ""
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
		filled := int((recoveryCoveredSectors(m) * uint64(width)) / m.Recovery.TotalSectors)
		if filled > width {
			filled = width
		}
		return "[" + strings.Repeat("=", filled) + strings.Repeat(".", width-filled) + "]"
	}

	return renderUnicodeProgressBar(width, recoveryCoveredSectors(m), m.Recovery.TotalSectors)
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
			return remaining + "  ETA estimating..."
		}
		return remaining
	}
	return remaining + "  ETA " + eta
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
