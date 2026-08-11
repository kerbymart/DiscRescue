package app

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

func renderShell(m Model, body []string, tier layoutTier) string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	layout := layoutFor(m.Width, m.Height)
	header := shellHeader(theme, layout.Width)
	divider := theme.Divider.Render(strings.Repeat("─", maxInt(1, layout.Width)))

	lines := []string{header, divider, theme.Accent.Render(pageTitle(m))}
	if subtitle := pageSubtitle(m.Page); tier != layoutCompact && m.Height >= 28 && subtitle != "" {
		lines = append(lines, theme.Muted.Render(subtitle))
	}
	lines = append(lines, "")
	prefix := flattenLines(lines)
	body = flattenLines(body)

	status := []string(nil)
	if showStatusLine(m.Page, tier) && m.Notice != nil && strings.TrimSpace(m.Notice.Text) != "" {
		status = flattenLines([]string{renderNotice(theme, *m.Notice, layout.Width)})
	}

	footer := renderFooter(m, layout.Width, tier)
	if m.Page != PageRecovering && m.Width > 0 && (m.DriveList.Width() > 0 || m.ActionList.Width() > 0 || m.DetailsViewport.Width() > 1) {
		helpView := FooterHelp(false)
		helpView.SetWidth(layout.Width)
		footer = helpView.View(pageHelpForModel(m))
	}
	suffix := append(status, divider, footer)
	suffix = flattenLines(suffix)
	lines = append(prefix, body...)
	lines = append(lines, suffix...)
	content := strings.Join(lines, "\n")

	if m.Width >= 60 && m.Height >= 18 {
		frame := lipgloss.NewStyle().
			Width(shellOuterWidth(m.Width)).
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6D5DF5"))
		content = frame.Render(content)
		if m.Width > 0 {
			content = lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, content)
		}
	}
	return content + "\n"
}

func flattenLines(lines []string) []string {
	flat := make([]string, 0, len(lines))
	for _, line := range lines {
		flat = append(flat, strings.Split(line, "\n")...)
	}
	return flat
}

func shellHeader(theme Theme, width int) string {
	brand := theme.Accent.Render("◉ DISCRESCUE")
	badge := theme.Badge.Render("OPTICAL RECOVERY")
	return joinEdges(brand, badge, width)
}

func pageSubtitle(page Page) string {
	switch page {
	case PageDiscover, PageNoDrives, PageDiscoveryError, PageChooseDrive:
		if page == PageNoDrives {
			return "Connect or insert a readable optical drive, then retry discovery."
		}
		return "Select a readable optical drive to begin."
	case PageInspectingMedia, PagePriorProcessing:
		return "Identify the contents before choosing a safe action."
	case PageChooseAction:
		return "One primary decision at a time. Advanced recovery stays optional."
	case PageChooseOutput:
		return "Choose the image destination and keep its recovery map beside it."
	case PageReview:
		return "Confirm the drive, logical contents, and destination."
	case PageRecovering, PagePausing, PagePaused:
		return "Progress reflects durable recovery state."
	case PageStopConfirm, PageEjectConfirm:
		return "Saving progress keeps this recovery resumable."
	case PageSummary:
		return "Review what was recovered and the safest next action."
	case PageResumeJobs, PageHistory:
		return "Local history helps find matching recovery work."
	case PageDetails:
		return "Scrollable technical detail for the current recovery."
	default:
		return "Guided optical-disc recovery."
	}
}

func renderNotice(theme Theme, notice NoticeModel, width int) string {
	marker, style := "i", theme.AccentSoft
	switch notice.Severity {
	case SeverityWarning:
		marker, style = "△", theme.Warning
	case SeverityError:
		marker, style = "×", theme.Danger
	}
	text := marker + " " + notice.String()
	return style.MaxWidth(width).Render(text)
}

func joinEdges(left, right string, width int) string {
	space := width - lipgloss.Width(left) - lipgloss.Width(right)
	if space < 1 {
		return left
	}
	return left + strings.Repeat(" ", space) + right
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func shellWidth(text string) int { return lipgloss.Width(text) }
