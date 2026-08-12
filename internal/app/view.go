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
