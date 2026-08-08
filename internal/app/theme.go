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
	Card       lipgloss.Style
	CardFocus  lipgloss.Style
	Label      lipgloss.Style
	Badge      lipgloss.Style
	Divider    lipgloss.Style
}

func newTheme(monochrome bool, dark bool) Theme {
	if monochrome {
		plain := lipgloss.NewStyle()
		return Theme{Text: plain, Muted: plain.Faint(true), Accent: plain.Bold(true), AccentSoft: plain, Surface: plain, Border: plain, Focus: plain.Bold(true), Success: plain.Bold(true), Warning: plain.Bold(true), Danger: plain.Bold(true), Key: plain.Bold(true), Card: plain, CardFocus: plain.Bold(true), Label: plain.Bold(true), Badge: plain.Bold(true), Divider: plain.Faint(true)}
	}
	primary, secondary, text, muted, surface := "#A78BFA", "#67E8F9", "#F8F7FF", "#9690A8", "#1C1926"
	if !dark {
		primary, secondary, text, muted, surface = "#6D28D9", "#0E7490", "#241B35", "#6B6478", "#F5F1FF"
	}
	border := lipgloss.Color("#514766")
	return Theme{
		Text:       lipgloss.NewStyle().Foreground(lipgloss.Color(text)),
		Muted:      lipgloss.NewStyle().Foreground(lipgloss.Color(muted)),
		Accent:     lipgloss.NewStyle().Foreground(lipgloss.Color(primary)).Bold(true),
		AccentSoft: lipgloss.NewStyle().Foreground(lipgloss.Color(secondary)),
		Surface:    lipgloss.NewStyle().Background(lipgloss.Color(surface)).Padding(0, 1),
		Border:     lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border),
		Focus:      lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF8FF")).Background(lipgloss.Color("#6D28D9")).Bold(true),
		Success:    lipgloss.NewStyle().Foreground(lipgloss.Color("#5EE6A8")).Bold(true),
		Warning:    lipgloss.NewStyle().Foreground(lipgloss.Color("#F7C65B")).Bold(true),
		Danger:     lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6B8A")).Bold(true),
		Key:        lipgloss.NewStyle().Foreground(lipgloss.Color(primary)).Bold(true),
		Card:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(border).Padding(0, 1),
		CardFocus:  lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color(primary)).Padding(0, 1),
		Label:      lipgloss.NewStyle().Foreground(lipgloss.Color(secondary)).Bold(true),
		Badge:      lipgloss.NewStyle().Foreground(lipgloss.Color("#FFF8FF")).Background(lipgloss.Color("#6D28D9")).Padding(0, 1).Bold(true),
		Divider:    lipgloss.NewStyle().Foreground(border),
	}
}
