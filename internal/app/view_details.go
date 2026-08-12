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
