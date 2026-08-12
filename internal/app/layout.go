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

// Layout is the shell-provided rectangle for a page body.
type Layout struct {
	Width, Height int
	Tier          layoutTier
}

func layoutFor(width, height int) Layout {
	tier := layoutTierFor(width, height)
	contentHeight := height - 9 // shell header, title, notice, footer, and border budget
	if contentHeight < 1 {
		contentHeight = 1
	}
	return Layout{Width: contentWidth(width), Height: contentHeight, Tier: tier}
}

func shellOuterWidth(width int) int {
	if width <= 0 {
		return 80
	}
	outer := width - 4
	if outer > 110 {
		outer = 110
	}
	if outer < 40 {
		outer = width
	}
	return outer
}

func interactiveWidth(width int) int {
	value := contentWidth(width) - 4
	if value < 16 {
		return 16
	}
	return value
}
