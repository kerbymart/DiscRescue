package app

import "strings"

func progressBarFor(m Model, tier layoutTier) string {
	width := 40
	if tier == layoutMedium {
		width = 28
	}
	if tier == layoutCompact {
		width = 16
	}
	if width < 8 {
		width = 8
	}

	if m.Recovery.TotalSectors == 0 {
		return "[" + strings.Repeat(".", width) + "]"
	}

	if m.Monochrome || tier == layoutCompact {
		filled := int((recoveryCoveredSectors(m) * uint64(width)) / m.Recovery.TotalSectors)
		if filled > width {
			filled = width
		}
		return "[" + strings.Repeat("=", filled) + strings.Repeat(".", width-filled) + "]"
	}

	return renderUnicodeProgressBar(width, recoveryCoveredSectors(m), m.Recovery.TotalSectors)
}
func renderUnicodeProgressBar(width int, recovered, total uint64) string {
	if total == 0 {
		return "[" + strings.Repeat(progressEmptyCell, width) + "]"
	}

	scaled := (float64(recovered) / float64(total)) * float64(width)
	full := int(scaled)
	if full > width {
		full = width
	}

	partial := 0
	if full < width {
		remainder := scaled - float64(full)
		partial = int(remainder * 8)
	}

	var bar strings.Builder
	bar.Grow(width*3 + 2)
	bar.WriteString("[")
	if full > 0 {
		bar.WriteString(strings.Repeat(progressFullCell, full))
	}
	if partial > 0 && full < width {
		bar.WriteString(progressPartialCells[partial-1])
		full++
	}
	if full < width {
		bar.WriteString(strings.Repeat(progressEmptyCell, width-full))
	}
	bar.WriteString("]")
	return bar.String()
}
func unicodeProgressBar(width int, recovered, total uint64) string {
	if total == 0 {
		return "[" + strings.Repeat("░", width) + "]"
	}
	levels := []string{"▏", "▎", "▍", "▌", "▋", "▊", "▉", "█"}
	scaled := (float64(recovered) / float64(total)) * float64(width)
	full := int(scaled)
	if full > width {
		full = width
	}
	partial := 0
	if full < width {
		remainder := scaled - float64(full)
		partial = int(remainder * 8)
	}

	var bar strings.Builder
	bar.Grow(width*3 + 2)
	bar.WriteString("[")
	if full > 0 {
		bar.WriteString(strings.Repeat("█", full))
	}
	if partial > 0 && full < width {
		bar.WriteString(levels[partial-1])
		full++
	}
	if full < width {
		bar.WriteString(strings.Repeat("░", width-full))
	}
	bar.WriteString("]")
	return bar.String()
}
