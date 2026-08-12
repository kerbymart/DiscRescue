package app

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func clampIndex(index, length int) int {
	if length <= 0 || index < 0 {
		return 0
	}
	if index >= length {
		return length - 1
	}
	return index
}
func cardLines(theme Theme, title string, content []string, width int, focused bool) []string {
	inner := maxInt(12, width-4)
	style := theme.Card
	if focused {
		style = theme.CardFocus
	}
	body := strings.Join(content, "\n")
	if title != "" {
		body = theme.Label.Render(strings.ToUpper(title)) + "\n" + body
	}
	return strings.Split(style.Width(inner).Render(body), "\n")
}
func choiceMenu(theme Theme, options []string, selected, width int) []string {
	lines := make([]string, 0, len(options))
	for i, option := range options {
		marker := "  "
		style := theme.Text.Padding(0, 1)
		if i == selected {
			marker = "> "
			style = theme.Focus.Padding(0, 1).Width(maxInt(8, width-2))
		}
		lines = append(lines, style.Render(fitToWidth(marker+option, maxInt(8, width-4))))
	}
	return lines
}
func metricStrip(theme Theme, metrics [][2]string, width int) []string {
	if len(metrics) == 0 {
		return nil
	}
	gap := len(metrics) - 1
	cellWidth := (width - gap) / len(metrics)
	if cellWidth < 14 {
		lines := make([]string, 0, len(metrics))
		for _, metric := range metrics {
			lines = append(lines, theme.Label.Render(metric[0])+"  "+theme.Text.Render(metric[1]))
		}
		return lines
	}
	cards := make([]string, 0, len(metrics))
	for _, metric := range metrics {
		cards = append(cards, theme.Card.Width(cellWidth).Render(theme.Label.Render(strings.ToUpper(metric[0]))+"\n"+theme.Text.Render(metric[1])))
	}
	return strings.Split(lipgloss.JoinHorizontal(lipgloss.Top, intersperse(cards, " ")...), "\n")
}
func intersperse(values []string, separator string) []string {
	if len(values) < 2 {
		return values
	}
	out := make([]string, 0, len(values)*2-1)
	for i, value := range values {
		if i > 0 {
			out = append(out, separator)
		}
		out = append(out, value)
	}
	return out
}
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return []string{""}
	}
	return strings.Split(lipgloss.Wrap(text, width, ""), "\n")
}
func labeledLines(label, value string, width int) []string {
	prefix := label
	if len(prefix) < 11 {
		prefix += strings.Repeat(" ", 11-len(prefix))
	}
	return wrapTextWithPrefix(prefix, value, width)
}
func wrapTextWithPrefix(prefix, value string, width int) []string {
	if width <= len(prefix)+1 {
		return append([]string{prefix}, wrapText(value, width)...)
	}
	available := width - len(prefix) - 1
	if available < 8 {
		available = 8
	}
	wrapped := wrapText(value, available)
	lines := make([]string, 0, len(wrapped))
	for i, line := range wrapped {
		if i == 0 {
			lines = append(lines, prefix+" "+line)
			continue
		}
		lines = append(lines, strings.Repeat(" ", len(prefix)+1)+line)
	}
	return lines
}
func fitToWidth(text string, width int) string {
	if width <= 0 {
		return text
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	return ansi.Truncate(text, width, "…")
}
func summaryOptions(m Model) []string {
	options := make([]string, 0, 3)
	if retry := summaryRetryActionLabel(m); retry != "" {
		options = append(options, retry)
	}
	return append(options,
		"Exit",
		"Choose another drive",
	)
}
func summaryRetryActionLabel(m Model) string {
	if m.Recovery.DeferredSectors > 0 && m.Recovery.UnreadableSectors > 0 {
		return "Retry unresolved sectors"
	}
	if m.Recovery.DeferredSectors > 0 {
		return "Retry deferred sectors"
	}
	if m.Recovery.UnreadableSectors > 0 {
		return "Retry unreadable sectors"
	}
	return ""
}
func replaceExtension(path, extension string) string {
	if strings.HasSuffix(path, ".iso") {
		return strings.TrimSuffix(path, ".iso") + extension
	}
	return path + extension
}
func formatCount(value uint64) string {
	plain := fmt.Sprintf("%d", value)
	if len(plain) <= 3 {
		return plain
	}
	var groups []string
	for len(plain) > 3 {
		groups = append([]string{plain[len(plain)-3:]}, groups...)
		plain = plain[:len(plain)-3]
	}
	groups = append([]string{plain}, groups...)
	return strings.Join(groups, ",")
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
func recoveryOutputName(m Model) string {
	path := firstNonEmpty(m.Recovery.OutputPath, m.Setup.OutputPath)
	if path == "" || path == "Not chosen yet" {
		return ""
	}
	path = strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}
func recoveryTimeSummary(m Model, tier layoutTier) string {
	remaining := strings.TrimSpace(m.Recovery.Remaining)
	eta := strings.TrimSpace(m.Recovery.ETA)

	if tier == layoutCompact {
		return remaining
	}

	if remaining == "" && eta == "" {
		if tier == layoutFull {
			return "Estimating time remaining..."
		}
		return ""
	}
	if remaining == "" {
		return eta
	}
	if eta == "" {
		if tier == layoutFull {
			return remaining + "  ETA estimating..."
		}
		return remaining
	}
	return remaining + "  ETA " + eta
}
