package app

import "strings"

func renderResumeJobsPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.ResumeList.Width() > 0 && len(m.ResumeList.Items()) > 0 {
		return cardLines(theme, "Saved recoveries", strings.Split(m.ResumeList.View(), "\n"), width, true)
	}
	if len(m.ResumeJobs) == 0 {
		lines := wrapText("No resumable recoveries were found in the current output folder.", width)
		lines = append(lines, "")
		lines = append(lines, wrapText("Press Enter or Esc to go back.", width)...)
		return lines
	}

	lines := wrapText("Select a saved recovery that matches the current disc contents.", width)
	for i, job := range m.ResumeJobs {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, "")
		lines = append(lines, wrapText(prefix+job.OutputPath, width)...)
		if tier != layoutCompact {
			lines = append(lines, labeledLines("Resume", job.Detail, width)...)
		}
	}
	lines = append(lines, "")
	backPrefix := "  "
	if m.Cursor >= len(m.ResumeJobs) {
		backPrefix = "> "
	}
	lines = append(lines, wrapText(backPrefix+"Back", width)...)
	return lines
}
func renderHistoryPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	if m.HistoryList.Width() > 0 && len(m.HistoryList.Items()) > 0 {
		return cardLines(theme, "Processed media", strings.Split(m.HistoryList.View(), "\n"), width, true)
	}
	if len(m.HistoryItems) == 0 {
		lines := wrapText("No processed media were found in the current output folder.", width)
		lines = append(lines, "")
		lines = append(lines, wrapText("Press Enter or Esc to go back.", width)...)
		return lines
	}

	lines := wrapText("Browse saved recovery images and their local recovery maps.", width)
	for i, item := range m.HistoryItems {
		prefix := "  "
		if i == m.Cursor {
			prefix = "> "
		}
		lines = append(lines, "")
		lines = append(lines, wrapText(prefix+item.Title, width)...)
		if tier != layoutCompact {
			lines = append(lines, labeledLines("Status", item.Status, width)...)
			if item.ModifiedAt != "" {
				lines = append(lines, labeledLines("Updated", item.ModifiedAt, width)...)
			}
		}
	}
	lines = append(lines, "")
	backPrefix := "  "
	if m.Cursor >= len(m.HistoryItems) {
		backPrefix = "> "
	}
	lines = append(lines, wrapText(backPrefix+"Back", width)...)
	return lines
}
