package app

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
