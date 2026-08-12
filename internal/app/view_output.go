package app

import (
	"fmt"
	"strings"
)

func renderOutputPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	directory := m.Setup.OutputDirectory
	if directory == "" {
		directory = " "
	}
	fileName := m.Setup.OutputFileName
	if fileName == "" {
		fileName = " "
	}
	if tier == layoutCompact || m.Height < 28 {
		if m.Setup.OutputEditing && m.Setup.ActiveOutputField == OutputFieldDirectory {
			directory = m.DirectoryInput.View()
		}
		if m.Setup.OutputEditing && m.Setup.ActiveOutputField == OutputFieldFileName {
			fileName = m.FileNameInput.View()
		}
		lines := []string{
			compactFieldLine(theme, "Folder", directory, width, m.Cursor == 0),
			compactFieldLine(theme, "File name", fileName, width, m.Cursor == 1),
			fitToWidth("Full path  "+firstNonEmpty(m.Setup.OutputPath, "Not chosen yet"), width),
		}
		if tier != layoutCompact {
			lines = append(lines, fitToWidth("Space      "+m.Setup.FreeSpace, width))
		}
		lines = append(lines, choiceMenu(theme, []string{"Continue with this target"}, boolIndex(m.Cursor == 2), width)...)
		return lines
	}
	lines := []string{theme.Muted.Render("Use Enter to edit a field. Tab moves focus while editing.")}
	lines = append(lines, "")
	if m.Setup.OutputEditing && m.Setup.ActiveOutputField == OutputFieldDirectory {
		lines = append(lines, outputFieldLines(theme, "Folder", m.DirectoryInput.View(), width, true, true)...)
	} else {
		lines = append(lines, outputFieldLines(theme, "Folder", directory, width, m.Cursor == 0, false)...)
	}
	lines = append(lines, "")
	if m.Setup.OutputEditing && m.Setup.ActiveOutputField == OutputFieldFileName {
		lines = append(lines, outputFieldLines(theme, "File name", m.FileNameInput.View(), width, true, true)...)
	} else {
		lines = append(lines, outputFieldLines(theme, "File name", fileName, width, m.Cursor == 1, false)...)
	}
	metadata := []string{
		theme.Label.Render("FULL PATH") + "  " + firstNonEmpty(m.Setup.OutputPath, "Not chosen yet"),
		theme.Label.Render("FORMAT") + "     " + m.Setup.OutputFormat,
	}
	if m.Setup.DefaultPath != "" && m.Setup.DefaultPath != "Not chosen yet" {
		metadata = append(metadata, theme.Label.Render("SUGGESTED")+"  "+m.Setup.DefaultPath)
	}
	if m.Setup.ResumeReady {
		metadata = append(metadata, theme.Success.Render("RESUME")+"     "+firstNonEmpty(m.Setup.ResumeDetail, "This target can resume a previous recovery."))
	}
	if tier != layoutCompact {
		metadata = append(metadata, theme.Label.Render("SPACE")+"      "+m.Setup.FreeSpace)
	}
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Target", metadata, width, false)...)
	lines = append(lines, "")
	lines = append(lines, choiceMenu(theme, []string{"Continue with this target"}, boolIndex(m.Cursor == 2), width)...)
	return lines
}
func compactFieldLine(theme Theme, label, value string, width int, selected bool) string {
	line := fmt.Sprintf("%-10s %s", label, value)
	if selected {
		return theme.Focus.Width(width).Render("> " + fitToWidth(line, maxInt(8, width-2)))
	}
	return theme.Text.Render("  " + fitToWidth(line, maxInt(8, width-2)))
}
func boolIndex(selected bool) int {
	if selected {
		return 0
	}
	return -1
}
func outputFieldLines(theme Theme, label, value string, width int, selected, editing bool) []string {
	caption := theme.Label.Render(strings.ToUpper(label))
	if editing {
		caption += "  " + theme.Badge.Render("EDITING")
	}
	style := theme.Card
	if selected {
		style = theme.CardFocus
	}
	inner := maxInt(12, width-4)
	field := style.Width(inner).Render(value)
	return append([]string{caption}, strings.Split(field, "\n")...)
}
func renderReviewPage(m Model, width int, tier layoutTier) []string {
	theme := newTheme(m.Monochrome, m.DarkBackground)
	options := []string{
		firstNonEmpty(m.Setup.ActionLabel, "Start a new recovery"),
		"Change recovery method",
		"Edit output path",
		"Choose another drive",
	}
	if tier == layoutCompact || m.Height < 28 {
		lines := []string{
			"Drive   " + selectedDriveLabel(m),
			"Disc    " + discSummary(m),
			"Output  " + m.Setup.OutputPath,
			"Mode    " + firstNonEmpty(m.Setup.MethodLabel, "Balanced recovery"),
		}
		if m.Setup.ResumeReady {
			lines = append(lines, "Map     "+m.Setup.ResumeMapPath)
			if m.Setup.ResumeDetail != "" && tier != layoutCompact {
				lines = append(lines, wrapText(m.Setup.ResumeDetail, width)...)
			}
		}
		lines = append(lines, "")
		lines = append(lines, choiceMenu(theme, options, m.Cursor, width)...)
		return lines
	}
	context := []string{theme.Label.Render("DRIVE") + "  " + selectedDriveLabel(m), theme.Label.Render("DISC") + "   " + discSummary(m)}
	target := []string{theme.Label.Render("IMAGE") + "  " + m.Setup.OutputPath, theme.Label.Render("MODE") + "   " + firstNonEmpty(m.Setup.MethodLabel, "Balanced recovery")}
	if m.Setup.ResumeReady {
		target = append(target, theme.Success.Render("MAP")+"    "+m.Setup.ResumeMapPath)
	}
	lines := cardLines(theme, "Disc", context, width, false)
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Output", target, width, false)...)
	if m.Setup.ResumeDetail != "" {
		lines = append(lines, "")
		lines = append(lines, theme.Muted.Render(m.Setup.ResumeDetail))
	}
	lines = append(lines, "")
	lines = append(lines, cardLines(theme, "Ready", choiceMenu(theme, options, m.Cursor, width-4), width, true)...)
	return lines
}
