package app

import (
	lipgloss "charm.land/lipgloss/v2"
	"strings"
)

func renderShell(m Model, body []string, tier layoutTier) string {
	theme := newTheme(m.Monochrome, false)
	l := layoutFor(m.Width, m.Height)
	lines := []string{theme.Accent.Render("◉ DiscRescue") + theme.Muted.Render("  optical recovery"), ""}
	lines = append(lines, theme.Accent.Render(pageTitle(m)))
	lines = append(lines, body...)
	if showStatusLine(m.Page, tier) && m.Notice != nil && m.Notice.Text != "" {
		marker, style := "·", theme.Muted
		switch m.Notice.Severity {
		case SeverityWarning:
			marker, style = "△", theme.Warning
		case SeverityError:
			marker, style = "×", theme.Danger
		}
		lines = append(lines, "", style.Render(marker+" "+m.Notice.Text))
	}
	footer := renderFooter(m.Page, l.Width, tier)
	if m.Width > 0 && (m.DriveList.Width() > 0 || m.ActionList.Width() > 0 || m.DetailsViewport.Width() > 1) {
		helpView := FooterHelp(tier == layoutFull)
		helpView.SetWidth(l.Width)
		footer = helpView.View(pageHelp(m.Page))
	}
	lines = append(lines, "", theme.Key.Render(footer))
	content := strings.Join(lines, "\n")
	if m.Width >= 60 && m.Height >= 18 && (m.DriveList.Width() > 0 || m.ActionList.Width() > 0 || m.DetailsViewport.Width() > 1) {
		frame := lipgloss.NewStyle().
			Width(maxInt(20, m.Width-4)).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6D5DF5"))
		content = frame.Render(content)
	}
	return content + "\n"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func shellWidth(text string) int { return lipgloss.Width(text) }
