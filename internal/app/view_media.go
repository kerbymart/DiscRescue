package app

import "strings"

func renderInspectingMediaPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	lines := []string{theme.Accent.Render(m.LoadingSpinner.View() + " Reading the disc layout and identity")}
	if m.SelectedDrive.DisplayName != "" {
		lines = append(lines, "")
		lines = append(lines, theme.Label.Render("DRIVE")+"  "+m.SelectedDrive.DisplayName)
	}
	if m.SelectedDrive.Path != "" {
		lines = append(lines, theme.Label.Render("PATH")+"   "+m.SelectedDrive.Path)
	}
	lines = append(lines, "")
	lines = append(lines, theme.Muted.Render("DiscRescue is checking capacity, layout, and matching local work."))
	return cardLines(theme, "Media inspection", lines, width, true)
}
func renderPriorProcessing(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.PriorView.Kind == PriorProcessingNone || m.PriorView.Kind == PriorProcessingIndeterminate {
		return cardLines(theme, "Matching contents", wrapText(m.PriorView.HistoryLine, width-4), width, false)
	}

	result := append([]string{theme.Accent.Render(m.PriorView.Title)}, wrapText(strings.Join(m.PriorView.Body, " "), width-4)...)
	if m.PriorView.ImagePath != "" {
		result = append(result, "", theme.Label.Render("IMAGE")+"  "+m.PriorView.ImagePath)
	}
	if tier != layoutCompact && m.PriorView.CopyLabel != "" {
		result = append(result, theme.Label.Render("COPY")+"   "+m.PriorView.CopyLabel)
	}
	if tier == layoutFull && m.PriorView.LastSaved != "" {
		result = append(result, theme.Label.Render("SAVED")+"  "+m.PriorView.LastSaved)
	}
	if tier == layoutFull && m.PriorView.Recovered != "" {
		result = append(result, theme.Success.Render("RECOVERED")+"  "+m.PriorView.Recovered)
	}
	if tier == layoutFull && m.PriorView.UnreadableSectors != "" {
		result = append(result, theme.Danger.Render("UNREADABLE")+"  "+m.PriorView.UnreadableSectors)
	}
	lines := cardLines(theme, "Matching contents", result, width, false)
	if len(m.PriorView.Options) > 0 {
		lines = append(lines, "")
		lines = append(lines, cardLines(theme, "Next action", choiceMenu(theme, m.PriorView.Options, m.Cursor, width-4), width, true)...)
	}
	return lines
}
func renderActionList(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	lines := []string{}
	if m.ActionList.Width() > 0 && m.Height >= 24 {
		lines = append(lines, cardLines(theme, "Recovery actions", strings.Split(m.ActionList.View(), "\n"), width, true)...)
	} else {
		actions := []string{"Start a new recovery", "Resume an unfinished recovery", "Browse processed media", "Choose another drive"}
		lines = append(lines, cardLines(theme, "Recovery actions", choiceMenu(theme, actions, m.Cursor, width-4), width, true)...)
	}
	if tier != layoutCompact {
		lines = append(lines, "")
		context := []string{theme.Muted.Render(firstNonEmpty(m.PriorView.HistoryLine, "Checking this computer for matching saved work."))}
		if m.Identity.Detail != "" {
			context = append(context, "Disc: "+m.Identity.Detail)
		}
		if m.Height >= 30 {
			lines = append(lines, cardLines(theme, "Current media", context, width, false)...)
		} else {
			lines = append(lines, context...)
		}
	}
	return lines
}
