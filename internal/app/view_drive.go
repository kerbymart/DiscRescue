package app

import "strings"

func renderDeviceList(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.DriveList.Width() > 0 && len(m.DriveList.Items()) > 0 {
		return cardLines(theme, "Available drives", strings.Split(m.DriveList.View(), "\n"), width, true)
	}
	if len(m.Devices) == 0 {
		return cardLines(theme, "Available drives", []string{"No usable optical drives found."}, width, true)
	}
	lines := make([]string, 0, len(m.Devices)*2)
	for i, device := range m.Devices {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, prefix+device.DisplayName, "  "+device.Path+" · "+device.Status)
	}
	return cardLines(theme, "Available drives", lines, width, true)
}
func renderNoDrivesPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	return cardLines(theme, "Next step", []string{
		"Connect an optical drive or insert a readable CD/DVD.",
		theme.Muted.Render("Press Enter to retry discovery."),
	}, width, false)
}
func renderDiscoveryErrorPage(m Model, width int) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	return cardLines(theme, "Discovery needs attention", []string{
		"Press Enter to retry discovery.",
		theme.Muted.Render("Press q to quit."),
	}, width, false)
}
func selectedDriveLabel(m Model) string {
	if strings.TrimSpace(m.SelectedDrive.DisplayName) != "" {
		return m.SelectedDrive.DisplayName
	}
	if m.Cursor >= 0 && m.Cursor < len(m.Devices) {
		return m.Devices[m.Cursor].DisplayName
	}
	if len(m.Devices) > 0 {
		return m.Devices[0].DisplayName
	}
	return "not selected"
}
func discSummary(m Model) string {
	if m.Identity.Detail != "" {
		return m.Identity.Detail
	}
	return "not identified"
}
