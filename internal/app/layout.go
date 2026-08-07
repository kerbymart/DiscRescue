package app

// Layout is the shell-provided rectangle for a page body.
type Layout struct {
	Width, Height int
	Tier          layoutTier
}

func layoutFor(width, height int) Layout {
	tier := layoutTierFor(width, height)
	contentHeight := height - 7 // brand, title, status, and footer budget
	if contentHeight < 1 {
		contentHeight = 1
	}
	return Layout{Width: contentWidth(width), Height: contentHeight, Tier: tier}
}
