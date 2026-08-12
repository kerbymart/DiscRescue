package app

import (
	"fmt"
	"strings"
)

func renderSummaryPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	statusMarker := "✓"
	statusStyle := theme.Success
	if m.Recovery.UnreadableSectors > 0 {
		statusMarker = "×"
		statusStyle = theme.Danger
	} else if m.Recovery.DeferredSectors > 0 || strings.Contains(strings.ToLower(m.Recovery.Status), "paused") {
		statusMarker = "△"
		statusStyle = theme.Warning
	}
	if tier == layoutCompact {
		lines := []string{
			fitToWidth(statusStyle.Render(statusMarker+" "+m.Recovery.Status), width),
			fitToWidth("Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath), width),
			fmt.Sprintf("Recovered  %s / %s", formatCount(m.Recovery.RecoveredSectors), formatCount(m.Recovery.TotalSectors)),
		}
		lines = append(lines, choiceMenu(theme, summaryOptions(m), m.Cursor, width)...)
		return lines
	}
	if m.Height < 28 {
		lines := []string{fitToWidth(statusStyle.Render(statusMarker+" "+m.Recovery.Status), width)}
		if m.Recovery.UnreadableSectors > 0 {
			lines = append(lines, fitToWidth(fmt.Sprintf("%s sectors could not be recovered.", formatCount(m.Recovery.UnreadableSectors)), width))
		}
		lines = append(lines,
			fitToWidth("Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath), width),
			fitToWidth("Map        "+firstNonEmpty(m.Summary.MapPath, replaceExtension(m.Recovery.OutputPath, ".drmap")), width),
			fmt.Sprintf("Recovered  %s / %s", formatCount(m.Recovery.RecoveredSectors), formatCount(m.Recovery.TotalSectors)),
		)
		if m.Summary.Duration != "" {
			lines = append(lines, fitToWidth("Duration   "+m.Summary.Duration, width))
		}
		lines = append(lines, choiceMenu(theme, summaryOptions(m), m.Cursor, width)...)
		return lines
	}
	result := []string{statusStyle.Render(statusMarker + " " + m.Recovery.Status), ""}
	if m.Recovery.UnreadableSectors == 0 {
		result = append(result, "Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath))
		result = append(result,
			fitToWidth(fmt.Sprintf("Recovered  %s of %s sectors", formatCount(m.Recovery.RecoveredSectors), formatCount(m.Recovery.TotalSectors)), width),
		)
		if m.Recovery.DeferredSectors > 0 {
			result = append(result, fitToWidth(fmt.Sprintf("Deferred   %s sectors remain for a later pass", formatCount(m.Recovery.DeferredSectors)), width))
		}
		if tier != layoutCompact && m.Summary.Duration != "" {
			result = append(result, fitToWidth("Duration   "+m.Summary.Duration, width))
		}
	} else {
		result = append(result,
			fitToWidth(fmt.Sprintf("%s sectors could not be recovered.", formatCount(m.Recovery.UnreadableSectors)), width),
			fitToWidth("The image and map are complete enough to inspect or retry later.", width),
			"",
		)
		result = append(result, "Image      "+firstNonEmpty(m.Summary.ImagePath, m.Recovery.OutputPath))
		if tier != layoutCompact {
			result = append(result, "Map        "+firstNonEmpty(m.Summary.MapPath, replaceExtension(m.Recovery.OutputPath, ".drmap")))
			if history := m.Summary.CatalogStatus.SummaryLine(); history != "" {
				result = append(result, "History    "+history)
			}
		}
	}
	lines := cardLines(theme, "Result", result, width, false)
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Next action", choiceMenu(theme, summaryOptions(m), m.Cursor, width-4), width, true)...)
	return lines
}
