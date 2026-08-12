package app

type layoutTier uint8

const (
	layoutFull layoutTier = iota
	layoutMedium
	layoutCompact
	layoutTooSmall
)

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
