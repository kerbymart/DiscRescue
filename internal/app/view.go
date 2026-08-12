package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
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
		if m.Page == PageRecovering || m.Page == PagePausing || m.Page == PagePaused || m.Page == PageStopConfirm || m.Page == PageDetails {
			lines = append(lines, "Recovery continues while you resize the window.")
		}
		view := tea.NewView(strings.Join(lines, "\n") + "\n")
		view.WindowTitle = "DiscRescue"
		view.AltScreen = usesAltScreen(m.Page)
		return view
	}

	view := tea.NewView(renderPage(m, tier))
	view.WindowTitle = "DiscRescue"
	view.AltScreen = usesAltScreen(m.Page)
	return view
}

func renderPage(m Model, tier layoutTier) string {
	width := contentWidth(m.Width)
	return renderShell(m, renderPageBody(m, width, tier), tier)
}

func renderPageBody(m Model, width int, tier layoutTier) []string {
	if body, ok := renderCompactPageBody(m, width, tier); ok {
		for i := range body {
			body[i] = fitToWidth(body[i], width)
		}
		return body
	}

	switch m.Page {
	case PageDiscover:
		return renderDiscoveryPage(m, width)
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
		return renderAdvancedPage(m, width)
	case PageAbout:
		return renderAboutPage(m, width)
	default:
		return nil
	}
}
