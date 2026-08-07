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
		lines = append(lines, "", theme.Muted.Render("Status: ")+theme.Text.Render(m.Notice.Text))
	}
	lines = append(lines, "", theme.Key.Render(renderFooter(m.Page, l.Width, tier)))
	return strings.Join(lines, "\n") + "\n"
}

func shellWidth(text string) int { return lipgloss.Width(text) }
