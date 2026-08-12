package app

import "discrescue/internal/buildinfo"

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

func renderCompactAboutPage(m Model, width int) ([]string, bool) {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	return []string{
		theme.Accent.Render("\u25c9 DiscRescue"),
		"Guided optical-disc recovery.",
		"Version " + buildinfo.Version,
		"Commit " + buildinfo.Commit,
	}, true
}
