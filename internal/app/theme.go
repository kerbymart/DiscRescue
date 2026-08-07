package app

import lipgloss "charm.land/lipgloss/v2"

// Theme contains the semantic presentation roles used by DiscRescue pages.
// Labels and markers remain meaningful when the terminal removes color.
type Theme struct {
	Text       lipgloss.Style
	Muted      lipgloss.Style
	Accent     lipgloss.Style
	AccentSoft lipgloss.Style
	Surface    lipgloss.Style
	Border     lipgloss.Style
	Focus      lipgloss.Style
	Success    lipgloss.Style
	Warning    lipgloss.Style
	Danger     lipgloss.Style
	Key        lipgloss.Style
}

func newTheme(monochrome bool, dark bool) Theme {
	if monochrome {
		plain := lipgloss.NewStyle()
		return Theme{Text: plain, Muted: plain.Faint(true), Accent: plain.Bold(true), AccentSoft: plain, Surface: plain, Border: plain, Focus: plain.Bold(true), Success: plain.Bold(true), Warning: plain.Bold(true), Danger: plain.Bold(true), Key: plain.Bold(true)}
	}
	primary, secondary := "62", "99"
	if !dark {
		primary, secondary = "27", "54"
	}
	return Theme{
		Text: lipgloss.NewStyle(), Muted: lipgloss.NewStyle().Foreground(lipgloss.Color("245")),
		Accent:     lipgloss.NewStyle().Foreground(lipgloss.Color(primary)).Bold(true),
		AccentSoft: lipgloss.NewStyle().Foreground(lipgloss.Color(secondary)),
		Surface:    lipgloss.NewStyle().Padding(0, 1), Border: lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color(secondary)),
		Focus: lipgloss.NewStyle().Bold(true).Underline(true), Success: lipgloss.NewStyle().Foreground(lipgloss.Color("42")),
		Warning: lipgloss.NewStyle().Foreground(lipgloss.Color("214")), Danger: lipgloss.NewStyle().Foreground(lipgloss.Color("196")), Key: lipgloss.NewStyle().Foreground(lipgloss.Color(primary)).Bold(true),
	}
}
