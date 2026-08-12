package app

import "strings"

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

func renderCompactDetailsPage(m Model, width int) ([]string, bool) {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	preview := "No technical detail is available."
	if content := strings.TrimSpace(m.DetailsViewport.View()); content != "" {
		preview = strings.Split(content, "\n")[0]
	}
	return []string{
		theme.Accent.Render("Recovery details"),
		fitToWidth(preview, width),
		theme.Muted.Render("Use up/down to scroll; esc to return."),
	}, true
}

func detailsLinesForView(m Model) []string {
	if m.PreviousPage == PageHistory && m.Page == PageDetails {
		return append([]string(nil), m.Details.Lines...)
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
