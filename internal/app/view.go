package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) View() tea.View {
	if m.Width > 0 && m.Height > 0 && (m.Width < 40 || m.Height < 12) {
		return tea.NewView("Window too small. Resize to at least 40x12.")
	}

	width := contentWidth(m.Width)
	lines := []string{
		"DiscRescue",
		"",
		fmt.Sprintf("Initial screen: %s", m.Screen),
		fmt.Sprintf("Focus: %s", m.CurrentFocus),
	}

	if m.Height == 0 || m.Height >= 18 {
		lines = append(lines, "")
		lines = append(lines, wrapText(m.Notice, width)...)
	}

	lines = append(lines, "")
	lines = append(lines, wrapText(m.StatusLine, width)...)

	if m.Height == 0 || m.Height >= 12 {
		lines = append(lines, "")
		lines = append(lines, renderFooter(width))
	}

	content := strings.Join(lines, "\n") + "\n"
	view := tea.NewView(content)
	view.WindowTitle = "DiscRescue"
	return view
}

func contentWidth(width int) int {
	if width <= 0 {
		return 76
	}

	value := width - 4
	if value < 40 {
		value = 40
	}
	if value > 76 {
		value = 76
	}
	return value
}

func renderFooter(width int) string {
	return fitToWidth("enter Start  q Quit", width)
}

func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return []string{""}
	}

	var lines []string
	var current strings.Builder

	for _, token := range strings.Fields(text) {
		if current.Len() == 0 {
			appendWrappedToken(&lines, &current, token, width)
			continue
		}

		if current.Len()+1+len(token) <= width {
			current.WriteByte(' ')
			current.WriteString(token)
			continue
		}

		lines = append(lines, current.String())
		current.Reset()
		appendWrappedToken(&lines, &current, token, width)
	}

	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

func appendWrappedToken(lines *[]string, current *strings.Builder, token string, width int) {
	if len(token) <= width {
		current.WriteString(token)
		return
	}

	for len(token) > width {
		*lines = append(*lines, token[:width])
		token = token[width:]
	}
	current.WriteString(token)
}

func fitToWidth(text string, width int) string {
	if width <= 0 || len(text) <= width {
		return text
	}
	return text[:width]
}
